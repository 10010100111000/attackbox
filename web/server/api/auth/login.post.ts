import { defineEventHandler, readBody, createError, getRequestHeader } from 'h3';
import { protectAuth } from '../../services/auth-protection';
import { db } from '../../db';
import { users, authSessions } from '../../db/schema';
import { eq } from 'drizzle-orm';
import {
  verifyPassword,
  generateSecureToken,
  hashToken,
  setSessionCookie,
  getEventIp,
} from '../../utils/auth';

export default defineEventHandler(async (event) => {
  const body = await readBody(event);
  const rawEmail = typeof body?.email === 'string' ? body.email.trim().toLowerCase() : '';
  const password = typeof body?.password === 'string' ? body.password : '';
  const rememberMe = Boolean(body?.rememberMe);

  if (!rawEmail || rawEmail.length > 254 || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(rawEmail) || !password || password.length > 1024) {
    throw createError({
      statusCode: 400,
      statusMessage: '请输入邮箱和密码',
    });
  }

  const clearRateLimit = await protectAuth(event, 'login', rawEmail, body?.turnstileToken);
  const [user] = await db
    .select()
    .from(users)
    .where(eq(users.email, rawEmail))
    .limit(1);

  if (!user || !user.passwordHash) {
    throw createError({
      statusCode: 401,
      statusMessage: '邮箱或密码错误',
    });
  }

  const isPasswordValid = await verifyPassword(user.passwordHash, password);
  if (!isPasswordValid) {
    throw createError({
      statusCode: 401,
      statusMessage: '邮箱或密码错误',
    });
  }

  // 账号生命周期状态校验
  if (user.status === 'pending') {
    throw createError({
      statusCode: 403,
      statusMessage: '账号尚未激活，请查收验证邮件以完成激活',
    });
  }

  if (user.status === 'suspended') {
    throw createError({
      statusCode: 403,
      statusMessage: '账号已被暂停使用，请联系管理员',
    });
  }

  // 签发会话凭证
  const rawSessionToken = generateSecureToken();
  const tokenHash = hashToken(rawSessionToken);
  const now = new Date();
  const expiresAt = new Date(
    now.getTime() + (rememberMe ? 30 * 24 * 60 * 60 * 1000 : 24 * 60 * 60 * 1000)
  );

  await db.transaction(async tx => {
    // Password verification is asynchronous: lock and recheck before issuing a session.
    const [current] = await tx.select().from(users).where(eq(users.id, user.id)).for('update');
    if (!current || current.status !== 'active' || current.passwordHash !== user.passwordHash) {
      throw createError({ statusCode: 403, statusMessage: '账号状态已变化，请重新登录' });
    }
    await tx.insert(authSessions).values({
      userId: user.id, tokenHash, expiresAt, ip: getEventIp(event),
      userAgent: getRequestHeader(event, 'user-agent')?.slice(0, 512),
    });
    await tx.update(users).set({ lastLoginAt: now }).where(eq(users.id, user.id));
  });

  await clearRateLimit();
  // 写入 HttpOnly Cookie
  setSessionCookie(event, rawSessionToken, rememberMe);

  return {
    user: {
      id: user.id,
      email: user.email,
      displayName: user.displayName,
      role: user.role,
      status: user.status,
    },
  };
});
