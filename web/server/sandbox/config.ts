import { createPrivateKey, createPublicKey, timingSafeEqual, type KeyObject } from 'node:crypto'
import type { SandboxPolicy } from '../../shared/sandbox.ts'
export interface NodeConfig { id: string; label: string; publicDataUrl: string; maxSlots: number }
export interface SandboxConfig { enabled: boolean; configurationError: string | null; secret: string; publicKey: string; signingKey: KeyObject | null; schedulingEnabled: boolean; policy: SandboxPolicy; nodes: NodeConfig[] }
export const defaultPolicy: SandboxPolicy = {
  resources: { cpu_weight: 100, memory_max_mb: 2048, memory_swap_max_mb: 0, pids_max: 512, io_write_bytes_sec: 20971520, io_write_iops: 500 },
  storage: { tmpfs_tmp_size: '512M', tmpfs_run_size: '256M' },
  security: { read_only_proc_sys: true, mask_sysrq: true }
}
function bounded(value: unknown, fallback: number, min: number, max: number) {
  const number = value === undefined ? fallback : Number(value)
  if (!Number.isSafeInteger(number) || number < min || number > max) throw new Error('沙箱运行策略超出允许范围')
  return number
}
const identifier = /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/

// This is the browser-facing endpoint, never a placeholder for a listener address.
export function publicGatewayUrl(value: unknown): string {
  if (typeof value !== 'string' || !/^wss?:\/\/[^/]/i.test(value) || /[\s\u0000-\u001f\u007f]/.test(value)) {
    throw new Error('publicDataUrl 必须为完整的 ws(s)://host[:port] 地址')
  }
  const url = new URL(value)
  if (!['ws:', 'wss:'].includes(url.protocol) || url.username || url.password || url.search || url.hash || url.pathname !== '/' || url.port === '0') {
    throw new Error('publicDataUrl 仅支持 ws(s)://host[:port]，不能包含凭据、路径或查询参数')
  }
  if (['0.0.0.0', '[::]'].includes(url.hostname)) throw new Error('publicDataUrl 需要浏览器可达的地址，不能填写通配监听地址')
  return url.origin
}

export function sandboxConfig(env: NodeJS.ProcessEnv = process.env): SandboxConfig {
  const disabled: SandboxConfig = { enabled: false, configurationError: null, secret: '', publicKey: '', signingKey: null, schedulingEnabled: false, policy: structuredClone(defaultPolicy), nodes: [] }
  if (env.SANDBOX_CONTROL_ENABLED !== 'true') return disabled
  try {
    if (!env.SANDBOX_CLUSTER_SECRET || Buffer.byteLength(env.SANDBOX_CLUSTER_SECRET) < 32) throw new Error('SANDBOX_CLUSTER_SECRET 至少需要 32 字节')
    if (!/^[a-fA-F0-9]{64}$/.test(env.SANDBOX_SIGNING_SEED_HEX || '')) throw new Error('SANDBOX_SIGNING_SEED_HEX 必须为 32 字节的十六进制 Ed25519 seed')
    const raw: unknown = JSON.parse(env.SANDBOX_NODES_JSON || '[]')
    if (!Array.isArray(raw) || raw.length < 1 || raw.length > 32) throw new Error('SANDBOX_NODES_JSON 需要配置 1–32 个节点')
    const nodes: NodeConfig[] = raw.map(node => {
      if (!node || !identifier.test(node.id) || typeof node.label !== 'string' || !node.label.trim() || node.label.length > 80 || typeof node.publicDataUrl !== 'string') throw new Error('节点必须包含有效的 id、label、publicDataUrl')
      const publicDataUrl = publicGatewayUrl(node.publicDataUrl)
      if (env.APP_BASE_URL && new URL(env.APP_BASE_URL).protocol === 'https:' && publicDataUrl.startsWith('ws:')) {
        throw new Error('HTTPS 网站的 publicDataUrl 必须使用 wss://，避免浏览器阻止混合内容连接')
      }
      return { id: node.id, label: node.label.trim(), publicDataUrl, maxSlots: bounded(node.maxSlots, 4, 1, 250) }
    })
    if (new Set(nodes.map(n => n.id)).size !== nodes.length) throw new Error('节点 id 不能重复')
    const key = createPrivateKey({ key: Buffer.concat([Buffer.from('302e020100300506032b657004220420', 'hex'), Buffer.from(env.SANDBOX_SIGNING_SEED_HEX!, 'hex')]), format: 'der', type: 'pkcs8' })
    const publicKey = createPublicKey(key).export({ type: 'spki', format: 'der' }).subarray(-32).toString('hex')
    const policy = structuredClone(defaultPolicy)
    policy.resources.memory_max_mb = bounded(env.SANDBOX_MEMORY_MB, 2048, 128, 65536)
    policy.resources.pids_max = bounded(env.SANDBOX_PIDS_MAX, 512, 32, 4096)
    return { enabled: true, configurationError: null, secret: env.SANDBOX_CLUSTER_SECRET, publicKey, signingKey: key, schedulingEnabled: env.SANDBOX_SCHEDULING_ENABLED === 'true', policy, nodes }
  } catch (error) {
    // JSON parsing and URL errors can include supplied values; never expose them.
    const safe = error instanceof Error && !(error instanceof SyntaxError) && !(error instanceof TypeError) ? error.message : '沙箱节点配置格式无效'
    return { ...disabled, configurationError: safe }
  }
}
export function authorizeWorker(config: SandboxConfig, headers: Headers): number | null {
  if (!config.enabled) return 503
  if (headers.has('origin')) return 403
  const auth = Buffer.from(headers.get('authorization') || '')
  const expected = Buffer.from(`Bearer ${config.secret}`)
  if (auth.length !== expected.length || !timingSafeEqual(auth, expected)) return 401
  if (!config.nodes.some(n => n.id === headers.get('x-node-id'))) return 403
  return null
}
