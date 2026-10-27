import { defineEventHandler, readBody, createError } from 'h3'
import { db } from '../../db'
import { users, emailTokens, authSessions } from '../../db/schema'
import { eq, and, gt, isNull } from 'drizzle-orm'
import { hashToken, hashPassword } from '../../utils/auth'

export default defineEventHandler(async (event) => {
  const body = await readBody(event)
  const rawToken = typeof body?.token === 'string' ? body.token.trim() : ''
  const newPassword =
    typeof body?.newPassword === 'string' ? body.newPassword : ''

  if (!rawToken) {
    throw createError({
      statusCode: 400,
      statusMessage: '重置凭证不能为空',
    })
  }

  if (!newPassword || newPassword.length < 6) {
    throw createError({
      statusCode: 400,
      statusMessage: '新密码至少需要 6 位',
    })
  }

  const tokenHash = hashToken(rawToken)
  const now = new Date()

  const [tokenRecord] = await db
    .select()
    .from(emailTokens)
    .where(
      and(
        eq(emailTokens.tokenHash, tokenHash),
        eq(emailTokens.purpose, 'reset_password'),
        gt(emailTokens.expiresAt, now),
        isNull(emailTokens.consumedAt),
      ),
    )
    .limit(1)

  if (!tokenRecord) {
    throw createError({
      statusCode: 400,
      statusMessage: '无效或已过期的重置凭证',
    })
  }

  const passwordHash = await hashPassword(newPassword)

  // 单事务更新密码、消费凭证并撤销该用户所有现存会话
  await db.transaction(async (tx) => {
    const [user] = await tx
      .select()
      .from(users)
      .where(eq(users.id, tokenRecord.userId))
      .for('update')
    if (
      !user ||
      user.status !== 'active' ||
      user.email !== tokenRecord.targetEmail
    )
      throw createError({
        statusCode: 400,
        statusMessage: '无效或已过期的重置凭证',
      })
    const [valid] = await tx
      .select()
      .from(emailTokens)
      .where(
        and(
          eq(emailTokens.id, tokenRecord.id),
          isNull(emailTokens.consumedAt),
          gt(emailTokens.expiresAt, new Date()),
        ),
      )
      .for('update')
    if (!valid)
      throw createError({
        statusCode: 400,
        statusMessage: '无效或已过期的重置凭证',
      })
    await tx
      .update(users)
      .set({
        passwordHash,
        updatedAt: now,
      })
      .where(eq(users.id, tokenRecord.userId))

    await tx
      .update(emailTokens)
      .set({ consumedAt: now })
      .where(
        and(
          eq(emailTokens.userId, user.id),
          eq(emailTokens.purpose, 'reset_password'),
          isNull(emailTokens.consumedAt),
        ),
      )

    // 撤销该用户的所有旧 Session
    await tx
      .update(authSessions)
      .set({ revokedAt: now })
      .where(
        and(
          eq(authSessions.userId, tokenRecord.userId),
          isNull(authSessions.revokedAt),
        ),
      )
  })

  return {
    message: '密码重置成功，旧会话已全部撤销，请使用新密码重新登录',
  }
})
