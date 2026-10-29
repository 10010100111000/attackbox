export interface RuntimePolicy {
  sandboxTtlMinutes: number
  sandboxIdleMinutes: number
  targetTtlMinutes: number
  lockVersion: number
}
export const defaultRuntimePolicy: RuntimePolicy = {
  sandboxTtlMinutes: 60,
  sandboxIdleMinutes: 10,
  targetTtlMinutes: 60,
  lockVersion: 0
}
export function parseRuntimePolicy(value: unknown): RuntimePolicy {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw Error('运行策略格式无效')
  const v = value as Record<string, unknown>
  const integer = (key: string, min: number, max: number) => {
    const n = v[key]
    if (typeof n !== 'number' || !Number.isSafeInteger(n) || n < min || n > max)
      throw Error('时长需为 1–1440 分钟的整数，空闲时间不能大于使用时长')
    return n
  }
  const sandboxTtlMinutes = integer('sandboxTtlMinutes', 1, 1440)
  return {
    sandboxTtlMinutes,
    sandboxIdleMinutes: integer('sandboxIdleMinutes', 1, sandboxTtlMinutes),
    targetTtlMinutes: integer('targetTtlMinutes', 1, 1440),
    lockVersion: integer('lockVersion', 0, 2147483646)
  }
}
