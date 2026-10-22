import { defineEventHandler, readBody, createError } from 'h3';
import { db } from '../../db';
import { users, emailTokens } from '../../db/schema';
import { eq, and, gt, isNull } from 'drizzle-orm';
import { hashToken } from '../../utils/auth';

export default defineEventHandler(async (event) => {
  const body = await readBody(event);
  const rawToken = typeof body?.token === 'string' ? body.token.trim() : '';

  if (!rawToken) {
    throw createError({
      statusCode: 400,
      statusMessage: '激活凭证不能为空',
    });
  }

  const tokenHash = hashToken(rawToken);
  const now = new Date();

  const [tokenRecord] = await db
    .select()
    .from(emailTokens)
    .where(
      and(
        eq(emailTokens.tokenHash, tokenHash),
        eq(emailTokens.purpose, 'verify_email'),
        gt(emailTokens.expiresAt, now),
        isNull(emailTokens.consumedAt)
      )
    )
    .limit(1);

  if (!tokenRecord) {
    throw createError({
      statusCode: 400,
      statusMessage: '无效或已过期的激活凭证',
    });
  }

  // 单事务消费凭证并激活账号
  await db.transaction(async (tx) => {
    await tx
      .update(users)
      .set({
        status: 'active',
        emailVerifiedAt: now,
        updatedAt: now,
      })
      .where(eq(users.id, tokenRecord.userId));

    await tx
      .update(emailTokens)
      .set({ consumedAt: now })
      .where(eq(emailTokens.id, tokenRecord.id));
  });

  return {
    message: '邮箱验证成功，账号已激活，可以登录',
  };
});
