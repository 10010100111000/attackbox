import { createHash } from 'node:crypto'
import { createError, setResponseHeader, type H3Event } from 'h3'
import { client } from '../db'
import { clientIp } from '../utils/client-ip'
import { verifyTurnstile, type AuthChallengeAction } from '../utils/turnstile'

const digest = (value: unknown) => createHash('sha256').update(JSON.stringify(value)).digest('hex')

async function consume(event: H3Event, key: string, limit: number, seconds: number) {
  // One atomic statement keeps the allowance correct across concurrent requests/processes.
  const [row] = await client`
    insert into auth_rate_limits(key, attempts, window_at) values(${key}, 1, now())
    on conflict(key) do update set
      attempts = case when auth_rate_limits.window_at <= now() - ${seconds} * interval '1 second'
        then 1 else least(auth_rate_limits.attempts + 1, ${limit + 1}) end,
      window_at = case when auth_rate_limits.window_at <= now() - ${seconds} * interval '1 second'
        then now() else auth_rate_limits.window_at end
    returning attempts,
      greatest(1, ceil(extract(epoch from window_at + ${seconds} * interval '1 second' - now())))::int as retry
  `
  if (row.attempts > limit) {
    setResponseHeader(event, 'Retry-After', Number(row.retry))
    throw createError({ statusCode: 429, message: '尝试次数较多，请稍后再试' })
  }
}

export async function protectAuth(event: H3Event, action: AuthChallengeAction, email: string, token: unknown) {
  const ip = clientIp(event)
  // Bounded cleanup during requests, no new timer or IP/email plaintext storage.
  await client`delete from auth_rate_limits where window_at < now() - interval '1 day' and key in (
    select key from auth_rate_limits where window_at < now() - interval '1 day' limit 20
  )`
  await consume(event, digest(['auth-ip', ip]), 60, 300)
  await verifyTurnstile(token, action, ip || undefined)
  const key = digest(['login-pair', ip, email])
  if (action === 'login') await consume(event, key, 8, 900)
  return async () => {
    if (action === 'login') await client`delete from auth_rate_limits where key = ${key}`
  }
}
