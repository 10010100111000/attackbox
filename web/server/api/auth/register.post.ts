import { defineEventHandler, readBody, createError } from 'h3'
import { protectAuth } from '../../services/auth-protection'
import { assertRegistrationEmail } from '../../utils/registration-policy'
import { sendAccountEmail } from '../../services/account-email'

export default defineEventHandler(async event => {
  const body = await readBody(event)
  const email = typeof body?.email === 'string' ? body.email.trim().toLowerCase() : ''
  if (!email || email.length > 254 || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email))
    throw createError({ statusCode: 400, message: '请输入有效的邮箱地址' })
  assertRegistrationEmail(email)
  await protectAuth(event, 'register', email, body?.turnstileToken)
  return sendAccountEmail(email, 'verify_email')
})
