import { defineEventHandler, readBody, createError } from 'h3'
import { protectAuth } from '../../services/auth-protection'
import { db } from '../../db'
import { users } from '../../db/schema'
import { eq } from 'drizzle-orm'
import { assertRegistrationEmail } from '../../utils/registration-policy'
import { hashPassword } from '../../utils/auth'
import { configuredMailer, sendAccountEmail } from '../../services/account-email'

export default defineEventHandler(async event => {
  const body = await readBody(event)
  const rawEmail = typeof body?.email === 'string' ? body.email.trim().toLowerCase() : ''
  const password = typeof body?.password === 'string' ? body.password : ''
  const displayName =
    typeof body?.displayName === 'string' && body.displayName.trim()
      ? body.displayName.trim()
      : rawEmail.split('@')[0] || 'User'

  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
  if (!rawEmail || !emailRegex.test(rawEmail)) {
    throw createError({
      statusCode: 400,
      statusMessage: '请输入有效的邮箱地址'
    })
  }

  if (displayName.length > 64 || rawEmail.length > 254)
    throw createError({ statusCode: 400, statusMessage: '邮箱或昵称过长' })

  if (!password || password.length < 6 || password.length > 1024) {
    throw createError({
      statusCode: 400,
      statusMessage: '密码至少需要 6 位'
    })
  }

  assertRegistrationEmail(rawEmail)
  await protectAuth(event, 'register', rawEmail, body?.turnstileToken)
  configuredMailer()

  // 检查邮箱是否已被注册
  const [existingUser] = await db
    .select({ id: users.id, status: users.status })
    .from(users)
    .where(eq(users.email, rawEmail))
    .limit(1)

  if (existingUser) {
    throw createError({
      statusCode: 409,
      statusMessage: '该邮箱已被注册'
    })
  }

  const passwordHash = await hashPassword(password)
  // 保留 pending 账号以支持投递失败后的重新发送。
  try {
    await db.insert(users).values({
      email: rawEmail,
      displayName,
      passwordHash,
      role: 'user',
      status: 'pending'
    })
  } catch (error: any) {
    if (error?.code === '23505' || error?.cause?.code === '23505')
      throw createError({ statusCode: 409, statusMessage: '该邮箱已被注册' })
    throw error
  }

  await sendAccountEmail(rawEmail, 'verify_email')
  return { message: '注册成功，请查收验证邮件以完成账号激活' }
})
