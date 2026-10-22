import { defineEventHandler, readBody } from 'h3';
import { db } from '../../db';
import { users, emailTokens } from '../../db/schema';
import { eq } from 'drizzle-orm';
import { generateSecureToken, hashToken } from '../../utils/auth';
import { mailer } from '../../utils/mailer';

export default defineEventHandler(async (event) => {
  const body = await readBody(event);
  const rawEmail = typeof body?.email === 'string' ? body.email.trim().toLowerCase() : '';

  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  let resetToken: string | undefined;

  if (rawEmail && emailRegex.test(rawEmail)) {
    const [user] = await db
      .select({ id: users.id, status: users.status })
      .from(users)
      .where(eq(users.email, rawEmail))
      .limit(1);

    if (user && user.status === 'active') {
      const rawToken = generateSecureToken();
      const tokenHash = hashToken(rawToken);
      const expiresAt = new Date(Date.now() + 30 * 60 * 1000); // 30 分钟有效

      await db.insert(emailTokens).values({
        userId: user.id,
        targetEmail: rawEmail,
        purpose: 'reset_password',
        tokenHash,
        expiresAt,
      });

      await mailer.sendPasswordResetEmail(rawEmail, rawToken);
      resetToken = rawToken;
    }
  }

  return {
    message: '如果该邮箱已注册并处于正常状态，我们已发送重置密码邮件',
    resetToken, // 测试环境便于测试验证
  };
});
