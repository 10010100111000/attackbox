import { createError } from 'h3'
import { defaultRegistrationDomains, registrationEmailAllowed } from '../../shared/registration'
export function registrationDomains(env: NodeJS.ProcessEnv = process.env): string[] {
  if (env.AUTH_REGISTRATION_EMAIL_DOMAINS === undefined) return [...defaultRegistrationDomains]
  const domains = [...new Set(env.AUTH_REGISTRATION_EMAIL_DOMAINS.split(',').map(value => value.trim().toLowerCase()))]
  if (!domains.length || domains.length > 100 || domains.some(domain => domain.length > 253 || !/^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/.test(domain))) {
    throw createError({ statusCode: 503, message: '注册邮箱策略配置无效，请联系管理员' })
  }
  return domains
}
export function assertRegistrationEmail(email: string) {
  const domains = registrationDomains()
  if (!registrationEmailAllowed(email, domains)) throw createError({
    statusCode: 403,
    message: '该邮箱域名暂不支持注册。',
  })
}
export function canSendActivationEmail(email: string) {
  return registrationEmailAllowed(email, registrationDomains())
}
