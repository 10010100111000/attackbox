import { createHash } from 'node:crypto'

export interface MailConfig {
  apiKey: string
  from: string
  baseUrl: string
}
export function mailConfig(): MailConfig {
  const apiKey = process.env.RESEND_API_KEY?.trim() || ''
  const from = process.env.MAIL_FROM?.trim() || ''
  let url: URL
  try {
    url = new URL(process.env.APP_BASE_URL || '')
  } catch {
    throw new Error('mail_configuration_invalid')
  }
  if (
    !apiKey ||
    !from ||
    /[\r\n]/.test(from) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash ||
    url.pathname !== '/' ||
    (url.protocol !== 'https:' &&
      !(
        url.protocol === 'http:' &&
        ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)
      ))
  )
    throw new Error('mail_configuration_invalid')
  return { apiKey, from, baseUrl: url.origin }
}
export function createResendMailer(
  config: MailConfig,
  transport: typeof fetch = fetch,
) {
  async function send(to: string, token: string, mode: 'verify' | 'reset') {
    const link = new URL('/auth', config.baseUrl)
    link.searchParams.set('mode', mode)
    link.searchParams.set('token', token)
    const title =
      mode === 'verify' ? '验证你的 AttackBox 邮箱' : '重置你的 AttackBox 密码'
    const expiry = mode === 'verify' ? '24 小时' : '30 分钟'
    const escape = (s: string) =>
      s.replace(
        /[&<>"']/g,
        (c) =>
          ({
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            '"': '&quot;',
            "'": '&#39;',
          })[c]!,
      )
    try {
      const response = await transport('https://api.resend.com/emails', {
        method: 'POST',
        redirect: 'error',
        signal: AbortSignal.timeout(10000),
        headers: {
          Authorization: `Bearer ${config.apiKey}`,
          'Content-Type': 'application/json',
          'Idempotency-Key': createHash('sha256')
            .update(`${mode}:${token}`)
            .digest('hex'),
        },
        body: JSON.stringify({
          from: config.from,
          to: [to],
          subject: title,
          text: `${title}\n\n${link.href}\n\n链接在 ${expiry}内有效。若非本人操作，请忽略此邮件。`,
          html: `<h1>${title}</h1><p><a href="${escape(link.href)}">${mode === 'verify' ? '验证邮箱' : '重置密码'}</a></p><p>链接在 ${expiry}内有效。若非本人操作，请忽略此邮件。</p>`,
        }),
      })
      if (!response.ok) throw new Error('rejected')
      const result = (await response.json()) as { id?: unknown }
      if (typeof result.id !== 'string' || !result.id)
        throw new Error('invalid_response')
    } catch {
      throw new Error('mail_delivery_failed')
    }
  }
  return {
    sendVerificationEmail: (to: string, token: string) =>
      send(to, token, 'verify'),
    sendPasswordResetEmail: (to: string, token: string) =>
      send(to, token, 'reset'),
  }
}
