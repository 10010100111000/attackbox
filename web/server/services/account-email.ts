import { createError } from 'h3'
import { canSendActivationEmail } from '../utils/registration-policy'
import { client } from '../db'
import { createResendMailer, mailConfig, MailDeliveryError } from '../utils/mailer'
import { generateSecureToken, hashToken } from '../utils/auth'
import { registrationLockSeed } from './registration'
import { accountEmailCooldownSeconds } from '#shared/account-email'

export const emailResult = {
  message: `如果该邮箱符合条件，我们会发送邮件。请检查收件箱或垃圾邮件；再次申请请间隔至少 ${accountEmailCooldownSeconds} 秒。`
}
export function configuredMailer() {
  try {
    return createResendMailer(mailConfig())
  } catch {
    console.error('[mail] configuration invalid; check RESEND_API_KEY, MAIL_FROM and APP_BASE_URL')
    throw createError({
      statusCode: 503,
      statusMessage: '邮件服务暂不可用，请稍后重试'
    })
  }
}
export async function sendAccountEmail(email: string, purpose: 'verify_email' | 'reset_password') {
  if (purpose === 'verify_email' && !canSendActivationEmail(email)) return emailResult
  const mailer = configuredMailer()
  try {
    await client.begin(async tx => {
      if (purpose === 'verify_email')
        await tx`select pg_advisory_xact_lock(hashtextextended(${email}, ${registrationLockSeed}))`
      // Serializes issuance for this recipient, including concurrent requests.
      const [user] = await tx`select id,status,role from users where email=${email} for update`
      if (purpose === 'reset_password' && (!user || user.status !== 'active')) return
      if (purpose === 'verify_email' && user && (user.status !== 'pending' || user.role !== 'user'))
        return
      await tx`delete from email_tokens where id in (select id from email_tokens where expires_at < now() - interval '1 day' limit 20)`
      const [limits] =
        await tx`select count(*)::int as count, max(created_at) as latest from email_tokens where target_email=${email} and purpose=${purpose} and created_at>now()-interval '1 hour'`
      if (
        limits.count >= 5 ||
        (limits.latest &&
          Date.now() - new Date(limits.latest).getTime() < accountEmailCooldownSeconds * 1000)
      )
        return
      const token = generateSecureToken(),
        lifetime = purpose === 'verify_email' ? 86400 : 1800
      const [issued] =
        await tx`insert into email_tokens(user_id,target_email,purpose,token_hash,expires_at) values(${user?.id || null},${email},${purpose},${hashToken(token)},now()+${lifetime}*interval '1 second') returning id`
      if (purpose === 'verify_email') await mailer.sendVerificationEmail(email, token)
      else await mailer.sendPasswordResetEmail(email, token)
      await tx`update email_tokens set consumed_at=now() where target_email=${email} and purpose=${purpose} and consumed_at is null and id<>${issued.id}`
    })
  } catch (error) {
    // Log only internal classifications / numeric provider status. Provider
    // bodies and thrown messages can contain addresses, credentials or links.
    console.error('[mail] account email failed', {
      purpose,
      reason: error instanceof MailDeliveryError ? error.reason : 'account_transaction_failed',
      providerStatus: error instanceof MailDeliveryError ? error.providerStatus : undefined
    })
    if (purpose === 'reset_password') {
      return emailResult
    }
    throw createError({
      statusCode: 503,
      statusMessage: '邮件发送暂未完成，请稍后重新申请'
    })
  }
  return emailResult
}
