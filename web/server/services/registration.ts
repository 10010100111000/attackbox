import { createError } from 'h3'
import { client } from '../db'
import { hashPassword, hashToken } from '../utils/auth'
import { assertRegistrationEmail } from '../utils/registration-policy'

// Both issuance and completion serialize only this email, including addresses
// without a users row. No account or password is reserved before mailbox proof.
export const registrationLockSeed = 49027821
const invalidToken = () =>
  createError({ statusCode: 400, message: '验证链接无效或已过期，请重新申请注册' })

function tokenDigest(value: unknown) {
  if (typeof value !== 'string' || !/^[a-f0-9]{64}$/.test(value)) throw invalidToken()
  return hashToken(value)
}

export async function inspectRegistration(value: unknown) {
  const digest = tokenDigest(value)
  const [token] = await client`
    select target_email from email_tokens where token_hash=${digest} and purpose='verify_email'
      and consumed_at is null and expires_at>now()`
  if (!token) throw invalidToken()
  return { email: String(token.target_email) }
}

export async function completeRegistration(value: unknown, password: unknown) {
  const digest = tokenDigest(value)
  const { email } = await inspectRegistration(value)
  assertRegistrationEmail(email)
  if (typeof password !== 'string' || password.length < 6 || password.length > 1024)
    throw createError({ statusCode: 400, message: '密码长度需为 6–1024 位' })
  await client.begin(async tx => {
    await tx`select pg_advisory_xact_lock(hashtextextended(${email}, ${registrationLockSeed}))`
    const [user] = await tx`select id,status,role from users where email=${email} for update`
    const [token] = await tx`
      select id from email_tokens where token_hash=${digest} and target_email=${email}
        and purpose='verify_email' and consumed_at is null and expires_at>now() for update`
    if (!token) throw invalidToken()
    if (user && (user.status !== 'pending' || user.role !== 'user'))
      throw createError({ statusCode: 403, message: '此账号当前无法注册，请登录或联系管理员' })
    // Claim the one-time proof before expensive password work. Concurrent
    // submissions for the same link cannot all start Argon2 hashes.
    const passwordHash = await hashPassword(password)
    let id: string
    if (user) {
      id = String(user.id)
      await tx`update users set password_hash=${passwordHash}, status='active',
        email_verified_at=now(),updated_at=now() where id=${id}`
      await tx`update auth_sessions set revoked_at=now() where user_id=${id} and revoked_at is null`
    } else {
      const [created] =
        await tx`insert into users(email,display_name,password_hash,status,email_verified_at)
        values(${email},${email.split('@')[0]!.slice(0, 64)},${passwordHash},'active',now()) returning id`
      id = String(created!.id)
    }
    await tx`update email_tokens set user_id=${id},consumed_at=now()
      where target_email=${email} and purpose='verify_email' and consumed_at is null`
  })
  return { message: '账号创建成功，请使用新设置的密码登录', email }
}
