import { createError } from 'h3'

export interface TurnstileConfig {
  enabled: boolean
  siteKey: string
  secretKey: string
  hostname: string
}
export type AuthChallengeAction = 'login' | 'register'

export function turnstileConfig(env: NodeJS.ProcessEnv = process.env): TurnstileConfig {
  const siteKey = env.TURNSTILE_SITE_KEY?.trim() || ''
  const secretKey = env.TURNSTILE_SECRET_KEY?.trim() || ''
  if (!siteKey && !secretKey) return { enabled: false, siteKey: '', secretKey: '', hostname: '' }

  let origin: URL
  try {
    origin = new URL(env.APP_BASE_URL || '')
  } catch {
    throw createError({ statusCode: 503, message: '人机验证配置不完整，请联系管理员' })
  }
  const localHttp = origin.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(origin.hostname)
  if (!siteKey || !secretKey || siteKey.length > 256 || secretKey.length > 256
    || origin.username || origin.password || origin.pathname !== '/' || origin.search || origin.hash
    || (origin.protocol !== 'https:' && !localHttp)) {
    throw createError({ statusCode: 503, message: '人机验证配置不完整，请联系管理员' })
  }
  return { enabled: true, siteKey, secretKey, hostname: origin.hostname.toLowerCase() }
}

export async function verifyTurnstile(
  token: unknown,
  action: AuthChallengeAction,
  remoteIp?: string,
  config = turnstileConfig(),
  transport: typeof fetch = fetch,
) {
  if (!config.enabled) return
  if (typeof token !== 'string' || !token || token.length > 2048) {
    throw createError({ statusCode: 403, message: '请先完成人机验证' })
  }
  const body = new URLSearchParams({ secret: config.secretKey, response: token })
  if (remoteIp) body.set('remoteip', remoteIp)

  let result: { success?: boolean; hostname?: string; action?: string }
  try {
    const response = await transport('https://challenges.cloudflare.com/turnstile/v0/siteverify', {
      method: 'POST', body, redirect: 'error', signal: AbortSignal.timeout(10000),
    })
    if (!response.ok) throw Error('siteverify unavailable')
    result = await response.json()
  } catch {
    throw createError({ statusCode: 503, message: '人机验证暂不可用，请稍后重试' })
  }
  if (result?.success !== true || typeof result.hostname !== 'string'
    || result.hostname.toLowerCase() !== config.hostname || result.action !== action) {
    throw createError({ statusCode: 403, message: '人机验证已失效，请重新验证' })
  }
}
