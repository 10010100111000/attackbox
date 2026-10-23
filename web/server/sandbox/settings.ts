import type { SandboxNodeSettings } from '../../shared/sandbox.ts'
export function parseNodeSettings(value: unknown): SandboxNodeSettings {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('节点设置格式无效')
  const v = value as Record<string, unknown>
  if (typeof v.label !== 'string' || !v.label.trim() || v.label.trim().length > 80 || typeof v.draining !== 'boolean') throw new Error('请填写节点名称和调度状态')
  const integer = (name: string, min: number, max: number) => {
    const n = v[name]
    if (typeof n !== 'number' || !Number.isSafeInteger(n) || n < min || n > max) throw new Error('节点资源或时限超出允许范围')
    return n
  }
  const ttlSeconds = integer('ttlSeconds', 60, 86400)
  return { label: v.label.trim(), draining: v.draining, maxSlots: integer('maxSlots', 1, 250), ttlSeconds, idleSeconds: integer('idleSeconds', 60, ttlSeconds), memoryMb: integer('memoryMb', 128, 65536), pidsMax: integer('pidsMax', 32, 4096), lockVersion: integer('lockVersion', 0, 2147483646) }
}
