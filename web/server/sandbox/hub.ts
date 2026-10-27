import { randomBytes } from 'node:crypto'
import type { SandboxNodesResponse, SandboxNodeStatus, SandboxNodeSettings } from '../../shared/sandbox.ts'
import type { SandboxConfig } from './config.ts'
import type {ComposeInstance,ComposeSummary,ComposeSettings} from '../../shared/compose-labs.ts'
export interface ComposePayload {requested_at?:string;rebuild?:boolean;target_id:string;owner_id:string;preparation_id?:string;deployment_id?:string;bundle_id?:string;bundle_hash?:string;settings?:ComposeSettings|null;variables?:Record<string,string>;expires_at?:string}
export interface ComposeWorkerResult {success:boolean;boot_id:string;target_id:string;owner_id?:string;job_state?:string;deployment_id?:string;summary?:ComposeSummary;logs?:string[];error?:string;instance?:ComposeInstance}
interface ComposePending {ownerId:string;peerId:string;bootId:string;targetId:string;type:string;resolve:(value:ComposeWorkerResult)=>void;reject:(error:Error)=>void;timer:ReturnType<typeof setTimeout>}
export interface WorkerPeer { id: string; send: (data: string) => unknown; close: (code: number, reason: string) => unknown }
interface Connection { peer: WorkerPeer; nodeId: string; openedAt: number; registered: boolean }
interface NodeRecord { status: SandboxNodeStatus; connection?: Connection; seq: number; revision: number; heartbeatAt: number }
export interface WorkerResult { success: boolean; sandbox_id: string; boot_id?: string; inventory_revision?: number; expires_at?: number; sandbox_ip?: string; error?: string }
interface PendingRPC { peerId: string; nodeId: string; bootId: string; sandboxId: string; type: string; resolve: (value: WorkerResult) => void; reject: (error: Error) => void; timer: ReturnType<typeof setTimeout> }
const integer = (n: unknown, min = 0): n is number => Number.isSafeInteger(n) && (n as number) >= min
const text = (s: unknown, max = 128): s is string => typeof s === 'string' && s.length > 0 && s.length <= max
const timestamp = (n: unknown) => integer(n, 1) && n <= 8640000000000 ? new Date(n * 1000).toISOString() : null
export class SandboxHub {
  composeNodes=new Set<string>()
  private composePending=new Map<string,ComposePending>()

  onInventory?: (node: SandboxNodeStatus) => void
  onLateCreate?: (nodeId: string, result: WorkerResult) => void
  private pending = new Map<string, PendingRPC>()
  private records = new Map<string, NodeRecord>()
  private connections = new Map<string, Connection>()
  readonly config: SandboxConfig
  private now: () => number
  constructor(config: SandboxConfig, now = Date.now) {
    this.config = config; this.now = now
    for (const n of config.nodes) this.records.set(n.id, { status: { settings: { label: n.label, maxSlots: n.maxSlots, memoryMb: config.policy.resources.memory_max_mb, pidsMax: config.policy.resources.pids_max, draining: false, lockVersion: 0 }, id: n.id, label: n.label, publicDataUrl: n.publicDataUrl, state: 'offline', bootId: null, lastHeartbeatAt: null, version: null, capabilities: null, telemetry: null, inventoryRevision: null, sandboxes: [] }, seq: 0, revision: 0, heartbeatAt: 0 })
  }
  applySettings(nodeId: string, settings: SandboxNodeSettings) {
    const record = this.records.get(nodeId)
    if (!record) return
    if (settings.lockVersion < record.status.settings.lockVersion) return
    record.status.settings = structuredClone(settings); record.status.label = settings.label
  }
  open(peer: WorkerPeer, nodeId: string) {
    const record = this.records.get(nodeId)
    if (!this.config.enabled || !record) { peer.close(1008, 'Unknown worker'); return }
    const old = record.connection
    const connection = { peer, nodeId, openedAt: this.now(), registered: false }
    record.connection = connection; this.connections.set(peer.id, connection)
    record.status.state = 'registering'
    if (old) { this.connections.delete(old.peer.id); this.failPending(old.peer.id); old.peer.close(1000, 'Connection replaced') }
  }
  close(peer: WorkerPeer) {
    const composeNode=this.connections.get(peer.id)?.nodeId;if(composeNode)this.composeNodes.delete(composeNode)
    const connection = this.connections.get(peer.id)
    this.connections.delete(peer.id)
    this.failPending(peer.id)
    const record = connection && this.records.get(connection.nodeId)
    if (record && record.connection === connection) { record.connection = undefined; record.status.state = 'offline' }
  }
  private failPending(peerId: string) {
    for(const [id,p]of this.composePending)if(p.peerId===peerId){clearTimeout(p.timer);this.composePending.delete(id);p.reject(new Error('Worker connection lost'))}


    for (const [id, pending] of this.pending) if (pending.peerId === peerId) {
      clearTimeout(pending.timer); this.pending.delete(id); pending.reject(new Error('Worker connection lost'))
    }
  }
  rpc(nodeId: string, bootId: string, type: 'create_box' | 'destroy_box', payload: Record<string, unknown>, timeoutMs = 30000): Promise<WorkerResult> {
    this.sweep()
    const record = this.records.get(nodeId)
    if (!record?.connection || record.status.state !== 'online' || record.status.bootId !== bootId) return Promise.reject(new Error('Worker unavailable'))
    const connection = record.connection, id = randomBytes(16).toString('hex')
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => { this.pending.delete(id); reject(new Error('Worker acknowledgement timeout')) }, timeoutMs)
      this.pending.set(id, { peerId: connection.peer.id, nodeId, bootId, sandboxId: String(payload.sandbox_id), type: type + '_resp', resolve, reject, timer })
      try { connection.peer.send(JSON.stringify({ id, type, payload: { ...payload, boot_id: bootId } })) }
      catch { clearTimeout(timer); this.pending.delete(id); reject(new Error('Worker send failed')) }
    })
  }
  private reject(peer: WorkerPeer) { this.close(peer); peer.close(1008, 'Invalid control message') }
  composeRpc(nodeId:string,bootId:string,type:'compose_prepare'|'compose_prepare_status'|'compose_prepare_cancel'|'compose_start'|'compose_inspect'|'compose_stop',payload:ComposePayload,timeoutMs=125000):Promise<ComposeWorkerResult>{
    this.sweep();const record=this.records.get(nodeId)
    if(!record?.connection||record.status.state!=='online'||record.status.bootId!==bootId||!this.composeNodes.has(nodeId))return Promise.reject(new Error('Compose worker unavailable'))
    if(!/^[A-Za-z0-9_-]{1,64}$/.test(payload.target_id)||!/^[A-Za-z0-9_-]{1,64}$/.test(payload.owner_id))return Promise.reject(new Error('Invalid Compose identity'))
    const peer=record.connection.peer,id=randomBytes(16).toString('hex')
    return new Promise((resolve,reject)=>{const timer=setTimeout(()=>{this.composePending.delete(id);reject(new Error('Worker acknowledgement timeout'))},timeoutMs);this.composePending.set(id,{ownerId:payload.owner_id,peerId:peer.id,bootId,targetId:payload.target_id,type:type+'_resp',resolve,reject,timer});try{peer.send(JSON.stringify({id,type,payload:{...payload,boot_id:bootId}}))}catch{clearTimeout(timer);this.composePending.delete(id);reject(new Error('Worker send failed'))}})
  }
  private reply(peer: WorkerPeer, type: string, payload: object) { peer.send(JSON.stringify({ id: randomBytes(8).toString('hex'), type, payload })) }
  message(peer: WorkerPeer, raw: string) {
    const connection = this.connections.get(peer.id)
    if (!connection) return
    const record = this.records.get(connection.nodeId)!
    if (record.connection !== connection) return
    try {
      if (Buffer.byteLength(raw) > 1024 * 1024) { this.reject(peer); return }
      const msg = JSON.parse(raw), p = msg?.payload
      if (!text(msg?.id) || !text(msg?.type) || !p || typeof p !== 'object' || Array.isArray(p)) { this.reject(peer); return }
      if(['compose_prepare_resp','compose_prepare_status_resp','compose_prepare_cancel_resp','compose_start_resp','compose_inspect_resp','compose_stop_resp'].includes(msg.type)){
        if(!connection.registered||typeof p.success!=='boolean'||!text(p.target_id)||!text(p.boot_id)){this.reject(peer);return}
        const pending=this.composePending.get(msg.id);if(!pending)return
        if(pending.peerId!==peer.id||pending.bootId!==p.boot_id||pending.targetId!==p.target_id||pending.type!==msg.type||p.owner_id!==pending.ownerId){this.reject(peer);return}
        if(p.instance&&(p.instance.target_id!==pending.targetId||p.instance.owner_id!==pending.ownerId||!text(p.instance.state)||typeof p.instance.ready!=='boolean'||!Array.isArray(p.instance.services)||p.instance.services.length>16)){this.reject(peer);return}
        if(p.instance&&(!text(p.instance.readiness)||!['not_connected','connected','disconnected'].includes(p.instance.attachment)||p.instance.services.some((s:any)=>!s||!text(s.name,64)||!text(s.state,64)||!text(s.health,64)||!s.ips||typeof s.ips!=='object'||Array.isArray(s.ips)||Object.entries(s.ips).length>8||Object.entries(s.ips).some(([k,v])=>!text(k,128)||typeof v!=='string'||v.length>64)))){this.reject(peer);return}
        if(p.deployment_id!==undefined&&!/^dep_[a-f0-9]{64}$/.test(p.deployment_id)){this.reject(peer);return}
        if(p.logs!==undefined&&(!Array.isArray(p.logs)||p.logs.length>20||p.logs.some((v:unknown)=>typeof v!=='string'||v.length>300))){this.reject(peer);return}
        if(p.summary&&(!Array.isArray(p.summary.services)||p.summary.services.length>16||p.summary.services.some((s:any)=>!s||!text(s.name,64)||!text(s.image,128))||!Array.isArray(p.summary.networks)||p.summary.networks.length>8||p.summary.networks.some((n:unknown)=>!text(n,64))||!Array.isArray(p.summary.report)||p.summary.report.length>128||p.summary.report.some((v:unknown)=>typeof v!=='string'||v.length>300)||!text(p.summary.entry_service,64)||!text(p.summary.entry_network,64)||!integer(p.summary.entry_port,1)||p.summary.entry_port>65535)){this.reject(peer);return}
        const safe:ComposeWorkerResult={success:p.success,boot_id:p.boot_id,target_id:p.target_id,owner_id:p.owner_id,...(typeof p.error==='string'?{error:p.error.slice(0,200)}:{}),...(p.job_state?{job_state:String(p.job_state)}:{}),...(p.deployment_id?{deployment_id:p.deployment_id}:{}),...(p.logs?{logs:p.logs}:{})}
        if(p.summary)safe.summary={services:p.summary.services.map((s:any)=>({name:s.name,image:s.image})),networks:p.summary.networks,entry_service:p.summary.entry_service,entry_network:p.summary.entry_network,entry_port:p.summary.entry_port,report:p.summary.report}
        if(p.instance)safe.instance={target_id:p.instance.target_id,owner_id:p.instance.owner_id,state:p.instance.state,ready:p.instance.ready,readiness:p.instance.readiness,entry_address:typeof p.instance.entry_address==='string'?p.instance.entry_address.slice(0,128):undefined,attachment:p.instance.attachment,attacker_address:typeof p.instance.attacker_address==='string'?p.instance.attacker_address.slice(0,64):undefined,interface:typeof p.instance.interface==='string'&&/^lab[0-9]{1,3}$/.test(p.instance.interface)?p.instance.interface:undefined,services:p.instance.services.map((s:any)=>({name:s.name,state:s.state,health:s.health,ips:{...s.ips}})),expires_at:typeof p.instance.expires_at==='string'?p.instance.expires_at:undefined}
        clearTimeout(pending.timer);this.composePending.delete(msg.id);pending.resolve(safe)
      } else if (msg.type === 'create_box_resp'  || msg.type === 'destroy_box_resp') {
        if (!connection.registered || typeof p.success !== 'boolean' || !text(p.sandbox_id)) { this.reject(peer); return }
        const pending = this.pending.get(msg.id)
        if (pending) {
          if (pending.peerId !== peer.id || pending.nodeId !== connection.nodeId || pending.type !== msg.type || pending.sandboxId !== p.sandbox_id || (p.success && p.boot_id !== pending.bootId)) { this.reject(peer); return }
          clearTimeout(pending.timer); this.pending.delete(msg.id); pending.resolve(p)
        } else if (msg.type === 'create_box_resp' && p.success && p.boot_id === record.status.bootId) {
          this.onLateCreate?.(connection.nodeId, p)
        }
      } else if (msg.type === 'register') {
        const node = this.config.nodes.find(n => n.id === connection.nodeId)!, c = p.capabilities
        if (connection.registered || p.node_id !== node.id || !text(p.boot_id) || !text(p.version, 64) || p.public_data_url?.replace(/\/$/, '') !== node.publicDataUrl || !integer(p.data_port, 1) || p.data_port > 65535 || !c || !integer(c.cpu_cores, 1) || !integer(c.total_mem_mb, 1) || !text(c.os_version) || typeof c.has_kvm !== 'boolean') { this.reject(peer); return }
        if (record.status.bootId !== p.boot_id) {
          record.seq = 0; record.revision = 0; record.heartbeatAt = 0
          record.status.sandboxes = []; record.status.telemetry = null; record.status.inventoryRevision = null; record.status.lastHeartbeatAt = null
        }
        record.status.bootId = p.boot_id; record.status.version = p.version
        record.status.capabilities = { cpuCores: c.cpu_cores, totalMemoryMb: c.total_mem_mb, os: c.os_version, hasKvm: c.has_kvm }
        connection.registered = true; record.status.state = 'waiting'
        this.reply(peer, 'register_ack', { success: true, node_id: node.id, public_key: this.config.publicKey })
      } else if (msg.type === 'heartbeat') {
        if (!connection.registered || p.node_id !== connection.nodeId || p.boot_id !== record.status.bootId || !integer(p.seq, 1) || !integer(p.inventory_revision) || !timestamp(p.timestamp) || !Number.isFinite(p.load_avg) || p.load_avg < 0 || !integer(p.mem_available_mb) || !integer(p.disk_free_gb) || !integer(p.active_sandboxes) || p.active_sandboxes > 250) { this.reject(peer); return }
        const inventory = p.sandboxes ?? []
        if (!Array.isArray(inventory) || inventory.length !== p.active_sandboxes || inventory.some(s => !s || !/^sbx_[a-zA-Z0-9_-]{1,100}$/.test(s.id) || !['creating', 'running', 'stopping', 'cleanup_failed'].includes(s.state) || !timestamp(s.last_active_at) || (s.gui_running !== undefined && typeof s.gui_running !== 'boolean')) || new Set(inventory.map(s => s.id)).size !== inventory.length) { this.reject(peer); return }
        // Reconnects with the same boot retain the clock. An old inventory never wins.
        if (p.seq <= record.seq || p.inventory_revision < record.revision) return
        record.seq = p.seq; record.revision = p.inventory_revision; record.heartbeatAt = this.now()
        record.status.state = 'online'; record.status.lastHeartbeatAt = new Date(this.now()).toISOString()
        record.status.inventoryRevision = p.inventory_revision
        record.status.telemetry = { loadAverage: p.load_avg, availableMemoryMb: p.mem_available_mb, freeDiskGb: p.disk_free_gb }
        record.status.sandboxes = inventory.map(s => ({ id: s.id, state: s.state, lastActiveAt: timestamp(s.last_active_at), guiRunning: !!s.gui_running }))
        this.reply(peer, 'heartbeat_ack', { success: true })
        if(p.compose_labs?.available===true&&p.compose_labs?.network_verified===true&&p.compose_labs?.protocol_version===2)this.composeNodes.add(connection.nodeId)
        else this.composeNodes.delete(connection.nodeId)
        this.onInventory?.(structuredClone(record.status))
      } else { this.reject(peer) }
    } catch { this.reject(peer) }
  }
  sweep() {
    for (const record of this.records.values()) {
      const c = record.connection
      if (!c) continue
      const deadline = record.status.state === 'online' ? record.heartbeatAt + 15000 : c.openedAt + 10000
      if (this.now() >= deadline) { this.close(c.peer); c.peer.close(1001, 'Heartbeat timeout') }
    }
  }
  snapshot(): SandboxNodesResponse {
    this.sweep()
    return { enabled: this.config.enabled, configurationError: this.config.configurationError, schedulingEnabled: this.config.schedulingEnabled, observedAt: new Date(this.now()).toISOString(), nodes: structuredClone([...this.records.values()].map(r => r.status)) }
  }
  shutdown() { for (const c of [...this.connections.values()]) { this.close(c.peer); c.peer.close(1001, 'Control shutdown') } }
}
