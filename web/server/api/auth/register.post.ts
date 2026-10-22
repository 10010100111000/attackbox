import { defineEventHandler, readBody, createError } from 'h3';
import { db } from '../../db';
import { users, emailTokens } from '../../db/schema';
import { eq } from 'drizzle-orm';
import { hashPassword, generateSecureToken, hashToken } from '../../utils/auth';
import { mailer } from '../../utils/mailer';

export default defineEventHandler(async (event) => {
  const body = await readBody(event);
  const rawEmail = typeof body?.email === 'string' ? body.email.trim().toLowerCase() : '';
  const password = typeof body?.password === 'string' ? body.password : '';
  const displayName = typeof body?.displayName === 'string' && body.displayName.trim()
    ? body.displayName.trim()
    : rawEmail.split('@')[0] || 'User';

  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  if (!rawEmail || !emailRegex.test(rawEmail)) {
    throw createError({
      statusCode: 400,
      statusMessage: '请输入有效的邮箱地址',
    });
  }

  if (!password || password.length < 6) {
    throw createError({
      statusCode: 400,
      statusMessage: '密码至少需要 6 位',
    });
  }

  // 检查邮箱是否已被注册
  const [existingUser] = await db
    .select({ id: users.id, status: users.status })
    .from(users)
    .where(eq(users.email, rawEmail))
    .limit(1);

  if (existingUser) {
    throw createError({
      statusCode: 409,
      statusMessage: '该邮箱已被注册',
    });
  }

  const passwordHash = await hashPassword(password);
  const rawToken = generateSecureToken();
  const tokenHash = hashToken(rawToken);
  const expiresAt = new Date(Date.now() + 24 * 60 * 60 * 1000); // 24小时有效

  // 插入 pending 用户与 email_tokens
  const [newUser] = await db
    .insert(users)
    .values({
      email: rawEmail,
      displayName,
      passwordHash,
      role: 'user',
      status: 'pending',
    })
    .returning({ id: users.id, email: users.email });

  await db.insert(emailTokens).values({
    userId: newUser.id,
    targetEmail: rawEmail,
    purpose: 'verify_email',
    tokenHash,
    expiresAt,
  });

  await mailer.sendVerificationEmail(rawEmail, rawToken);

  return {
    message: '注册成功，请查收验证邮件以完成账号激活',
    verifyToken: rawToken, // 供自动化测试和无外部 SMTP 环境直接激活
  };
});
