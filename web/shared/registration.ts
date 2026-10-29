export const defaultRegistrationDomains = [
  'gmail.com',
  'googlemail.com',
  'outlook.com',
  'hotmail.com',
  'live.com',
  'msn.com',
  'icloud.com',
  'me.com',
  'mac.com',
  'qq.com',
  'foxmail.com',
  '163.com',
  '126.com',
  'yeah.net',
  'yahoo.com',
  'aliyun.com'
] as const

export function registrationEmailAllowed(email: string, domains: readonly string[]): boolean {
  const normalized = email.trim().toLowerCase()
  if (normalized.length > 254 || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(normalized)) return false
  return domains.includes(normalized.slice(normalized.lastIndexOf('@') + 1))
}
