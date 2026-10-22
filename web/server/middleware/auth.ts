import { defineEventHandler } from 'h3';
import { db } from '../db';
import { authSessions, users } from '../db/schema';
import { eq, and, gt, isNull } from 'drizzle-orm';
import { getSessionToken, hashToken } from '../utils/auth';

declare module 'h3' {
  interface H3EventContext {
    user?: {
      id: string;
      email: string;
      displayName: string;
      role: 'user' | 'admin';
      status: 'pending' | 'active' | 'suspended';
    };
    session?: {
      id: string;
      userId: string;
      expiresAt: Date;
    };
  }
}

export default defineEventHandler(async (event) => {
  const token = getSessionToken(event);
  if (!token) return;

  try {
    const tokenHash = hashToken(token);
    const now = new Date();

    const [record] = await db
      .select({
        sessionId: authSessions.id,
        userId: authSessions.userId,
        expiresAt: authSessions.expiresAt,
        user: {
          id: users.id,
          email: users.email,
          displayName: users.displayName,
          role: users.role,
          status: users.status,
        },
      })
      .from(authSessions)
      .innerJoin(users, eq(authSessions.userId, users.id))
      .where(
        and(
          eq(authSessions.tokenHash, tokenHash),
          gt(authSessions.expiresAt, now),
          isNull(authSessions.revokedAt),
          eq(users.status, 'active')
        )
      )
      .limit(1);

    if (record) {
      event.context.user = record.user as any;
      event.context.session = {
        id: record.sessionId,
        userId: record.userId,
        expiresAt: record.expiresAt,
      };

      // 异步更新 lastSeenAt
      db.update(authSessions)
        .set({ lastSeenAt: now })
        .where(eq(authSessions.id, record.sessionId))
        .catch(() => {});
    }
  } catch {
    // 忽略异常，未命中有效 session 时保持 context.user 为空
  }
});
