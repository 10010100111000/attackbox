import { randomBytes, sign } from 'node:crypto'
import { isIP } from 'node:net'
import type { SandboxHub, WorkerResult } from './hub.ts'
import type { SandboxInstance, SandboxNodeStatus, SandboxStatus, SandboxTicket } from '../../shared/sandbox.ts'
export class SandboxError extends Error {
  statusCode: number
  constructor(statusCode: number, message: string) { super(message); this.statusCode = statusCode }
}
interface InstanceRecord extends Omit<SandboxInstance, 'connected'> { userId: string; workerUserId: number; bootId: string; confirmedRevision: number | null }
export class SandboxController {
  private instances = new Map<string, InstanceRecord>()
  private byUser = new Map<string, string>()
  private stopping = new Map<string, Promise<void>>()
  private unknownCleanup = new Set<string>()
  private hub: SandboxHub
  private now: () => number
  private timeout: number
  constructor(hub: SandboxHub, now = Date.now, timeoutMs = 30000) {
    this.hub = hub; this.now = now; this.timeout = timeoutMs
    hub.onInventory = node => this.reconcile(node)
    hub.onLateCreate = (nodeId, result) => this.lateCreate(nodeId, result)
  }
  private nodes() { return this.hub.snapshot().nodes }
  private enabled() { return this.hub.config.enabled && this.hub.config.schedulingEnabled }
  private view(record: InstanceRecord): SandboxInstance {
    const node = this.nodes().find(n => n.id === record.nodeId)
    return structuredClone({ id: record.id, nodeId: record.nodeId, state: record.state, slot: record.slot, ip: record.ip, startedAt: record.startedAt, expiresAt: record.expiresAt, lastActiveAt: record.lastActiveAt, connected: node?.state === 'online' && node.bootId === record.bootId, policy: record.policy, error: record.error })
  }
  status(userId: string): SandboxStatus {
    const record = this.instances.get(this.byUser.get(userId) || '')
    return { available: this.enabled(), instance: record ? this.view(record) : null }
  }
  list() { return [...this.instances.values()].map(record => ({ ...this.view(record), userId: record.userId })) }
  private remove(record: InstanceRecord) {
    if (this.instances.get(record.id) !== record) return
    this.instances.delete(record.id)
    if (this.byUser.get(record.userId) === record.id) this.byUser.delete(record.userId)
  }
  private known(node: SandboxNodeStatus, id: string) {
    const record = this.instances.get(id)
    return record?.nodeId === node.id && record.bootId === node.bootId
  }
  private canSchedule(node: SandboxNodeStatus) {
    if (node.state !== 'online' || !node.bootId || !node.telemetry || node.telemetry.freeDiskGb < 5) return false
    if (node.sandboxes.some(box => !this.known(node, box.id))) return false
    const creating = [...this.instances.values()].filter(r => r.nodeId === node.id && r.bootId === node.bootId && r.state === 'creating').length
    return node.telemetry.availableMemoryMb - creating * this.hub.config.policy.resources.memory_max_mb >= this.hub.config.policy.resources.memory_max_mb + 256
  }
  async start(userId: string, workerUserId: number): Promise<SandboxInstance> {
    if (!this.enabled()) throw new SandboxError(503, '练习环境尚未开放')
    if (!Number.isSafeInteger(workerUserId) || workerUserId <= 0) throw new SandboxError(500, '沙箱身份无效')
    const previous = this.instances.get(this.byUser.get(userId) || '')
    if (previous) return this.view(previous)
    let selected: { node: SandboxNodeStatus; slot: number } | undefined
    for (const node of this.nodes()) {
      if (!this.canSchedule(node)) continue
      const occupied = new Set([...this.instances.values()].filter(r => r.nodeId === node.id && r.bootId === node.bootId).map(r => r.slot))
      const limit = this.hub.config.nodes.find(n => n.id === node.id)!.maxSlots
      for (let slot = 1; slot <= limit; slot++) if (!occupied.has(slot)) { selected = { node, slot }; break }
      if (selected) break
    }
    if (!selected) throw new SandboxError(503, '暂无可用节点或空闲资源，请稍后重试')
    const { node, slot } = selected
    const record: InstanceRecord = { id: 'sbx_' + randomBytes(12).toString('hex'), userId, workerUserId, nodeId: node.id, bootId: node.bootId!, state: 'creating', slot, ip: null, startedAt: new Date(this.now()).toISOString(), expiresAt: null, lastActiveAt: null, confirmedRevision: null, policy: structuredClone(this.hub.config.policy), error: null }
    // Reservation is synchronous and precedes RPC: concurrent starts share one instance.
    this.instances.set(record.id, record); this.byUser.set(userId, record.id)
    try {
      const result = await this.hub.rpc(node.id, record.bootId, 'create_box', { sandbox_id: record.id, user_id: workerUserId, slot_index: slot, ttl_seconds: this.hub.config.ttlSeconds, idle_seconds: this.hub.config.idleSeconds, policy: record.policy }, this.timeout)
      if (this.instances.get(record.id) !== record) throw new Error('Worker generation changed')
      if (!result.success || !Number.isSafeInteger(result.inventory_revision) || result.inventory_revision! < 1 || !Number.isSafeInteger(result.expires_at) || result.expires_at! * 1000 <= this.now() || result.expires_at! > 8640000000000 || !result.sandbox_ip || isIP(result.sandbox_ip) !== 4) throw new Error('Invalid create acknowledgement')
      record.confirmedRevision = result.inventory_revision!
      record.expiresAt = new Date(result.expires_at! * 1000).toISOString(); record.ip = result.sandbox_ip
      // Stop may have been requested while create was in flight. Never resurrect it.
      if (record.state === 'creating') record.state = 'running'
      return this.view(record)
    } catch {
      if (this.instances.get(record.id) === record) {
        record.state = 'cleanup_failed'; record.error = '启动结果尚未确认，正在核对并清理'
        void this.release(record).catch(() => {})
      }
      throw new SandboxError(502, '环境启动未完成，请刷新状态后重试')
    }
  }
  async stopOwned(userId: string, id: string) {
    const record = this.instances.get(id)
    if (!record) return
    if (record.userId !== userId) throw new SandboxError(404, '实例不存在')
    await this.release(record)
  }
  async stopAdmin(id: string) {
    const record = this.instances.get(id)
    if (!record) throw new SandboxError(404, '实例不存在或已释放')
    await this.release(record)
  }
  private release(record: InstanceRecord): Promise<void> {
    const pending = this.stopping.get(record.id)
    if (pending) return pending
    record.state = 'stopping'; record.error = null
    const task = (async () => {
      try {
        const result = await this.hub.rpc(record.nodeId, record.bootId, 'destroy_box', { sandbox_id: record.id, reason: 'platform_release' }, this.timeout)
        if (!result.success) throw new Error('Cleanup not confirmed')
        this.remove(record)
      } catch {
        if (this.instances.get(record.id) === record) { record.state = 'cleanup_failed'; record.error = '释放尚未确认，保留资源占用，等待节点核对' }
        throw new SandboxError(502, '释放尚未确认，请稍后刷新或重试')
      } finally { this.stopping.delete(record.id) }
    })()
    this.stopping.set(record.id, task)
    return task
  }
  private cleanUnknown(nodeId: string, bootId: string, id: string) {
    const key = `${nodeId}:${bootId}:${id}`
    if (this.unknownCleanup.has(key)) return
    this.unknownCleanup.add(key)
    void this.hub.rpc(nodeId, bootId, 'destroy_box', { sandbox_id: id, reason: 'unknown_after_control_restart' }, this.timeout).catch(() => {}).finally(() => this.unknownCleanup.delete(key))
  }
  private lateCreate(nodeId: string, result: WorkerResult) {
    if (!this.enabled() || !result.boot_id || !/^sbx_[a-zA-Z0-9_-]{1,100}$/.test(result.sandbox_id)) return
    const record = this.instances.get(result.sandbox_id)
    if (record && record.nodeId === nodeId && record.bootId === result.boot_id) {
      if (record.state === 'running') return // Duplicate acknowledgement, not an abandoned create.
      record.state = 'cleanup_failed'; record.error = '迟到的创建结果正在清理'
      void this.release(record).catch(() => {})
    } else this.cleanUnknown(nodeId, result.boot_id, result.sandbox_id)
  }
  private reconcile(node: SandboxNodeStatus) {
    if (!this.enabled()) return
    const alive = new Map(node.sandboxes.map(s => [s.id, s]))
    for (const record of this.instances.values()) {
      if (record.nodeId !== node.id) continue
      if (record.bootId !== node.bootId) { this.remove(record); continue }
      const box = alive.get(record.id); alive.delete(record.id)
      if (record.state === 'creating' || this.stopping.has(record.id)) continue
      if (!box && record.confirmedRevision !== null && node.inventoryRevision! >= record.confirmedRevision) { this.remove(record); continue }
      if (record.state === 'cleanup_failed' || box?.state === 'cleanup_failed') { void this.release(record).catch(() => {}); continue }
      if (box) {
        record.lastActiveAt = box.lastActiveAt
        if (box.state === 'stopping') record.state = 'stopping'
      }
    }
    for (const id of alive.keys()) this.cleanUnknown(node.id, node.bootId!, id)
  }
  ticket(userId: string, id: string): SandboxTicket {
    const record = this.instances.get(id)
    if (!this.enabled() || !record || record.userId !== userId) throw new SandboxError(404, '实例不存在或已回收')
    const view = this.view(record)
    if (record.state !== 'running' || !view.connected || !record.expiresAt || Date.parse(record.expiresAt) <= this.now()) throw new SandboxError(409, '实例尚未就绪或节点已断开，请刷新状态')
    const exp = Math.min(Math.floor(this.now() / 1000) + 60, Math.floor(Date.parse(record.expiresAt) / 1000))
    const claims = Buffer.from(JSON.stringify({ jti: randomBytes(16).toString('hex'), sub: record.workerUserId, sbx: record.id, typ: 'pty', exp }))
    const token = claims.toString('base64url') + '.' + sign(null, claims, this.hub.config.signingKey!).toString('base64url')
    return { url: this.hub.config.nodes.find(n => n.id === record.nodeId)!.publicDataUrl + '/ws/cli', ticket: token, expiresAt: new Date(exp * 1000).toISOString(), sandboxId: record.id }
  }
}
