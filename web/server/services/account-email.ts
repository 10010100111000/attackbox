import { createError } from 'h3'
import { client } from '../db'
import { createResendMailer, mailConfig } from '../utils/mailer'
import { generateSecureToken, hashToken } from '../utils/auth'

export const emailResult = {
  message:
    '如果该邮箱符合条件，我们会发送邮件。请检查收件箱或垃圾邮件；再次申请请间隔至少 60 秒。',
}
export function configuredMailer() {
  try {
    return createResendMailer(mailConfig())
  } catch {
    throw createError({
      statusCode: 503,
      statusMessage: '邮件服务暂不可用，请稍后重试',
    })
  }
}
export async function sendAccountEmail(
  email: string,
  purpose: 'verify_email' | 'reset_password',
) {
  const mailer = configuredMailer()
  try {
    await client.begin(async (tx) => {
      // Serializes issuance for this recipient, including concurrent requests.
      const [user] =
        await tx`select id,status from users where email=${email} for update`
      if (
        !user ||
        user.status !== (purpose === 'verify_email' ? 'pending' : 'active')
      )
        return
      const [limits] =
        await tx`select count(*)::int as count, max(created_at) as latest from email_tokens where user_id=${user.id} and purpose=${purpose} and created_at>now()-interval '1 hour'`
      if (
        limits.count >= 5 ||
        (limits.latest &&
          Date.now() - new Date(limits.latest).getTime() < 60000)
      )
        return
      const token = generateSecureToken(),
        lifetime = purpose === 'verify_email' ? 86400 : 1800
      const [issued] =
        await tx`insert into email_tokens(user_id,target_email,purpose,token_hash,expires_at) values(${user.id},${email},${purpose},${hashToken(token)},now()+${lifetime}*interval '1 second') returning id`
      if (purpose === 'verify_email')
        await mailer.sendVerificationEmail(email, token)
      else await mailer.sendPasswordResetEmail(email, token)
      await tx`update email_tokens set consumed_at=now() where user_id=${user.id} and purpose=${purpose} and consumed_at is null and id<>${issued.id}`
    })
  } catch {
    if (purpose === 'reset_password') {
      console.error('[mail] reset delivery failed')
      return emailResult
    }
    throw createError({
      statusCode: 503,
      statusMessage: '邮件发送暂未完成，请稍后重新申请',
    })
  }
  return emailResult
}
