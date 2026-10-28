import { defineEventHandler } from 'h3';
import { db } from '../db';
import { authSessions, users } from '../db/schema';
import { eq, and, gt, lt, isNull } from 'drizzle-orm';
import { getSessionToken, hashToken } from '../utils/auth';

const LAST_SEEN_UPDATE_INTERVAL_MS = 5 * 60 * 1000;

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
        lastSeenAt: authSessions.lastSeenAt,
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

      // 节流异步更新 lastSeenAt，避免每次请求触发写事务与 MVCC 死元组堆积
      const refreshThreshold = new Date(now.getTime() - LAST_SEEN_UPDATE_INTERVAL_MS);
      if (!record.lastSeenAt || record.lastSeenAt < refreshThreshold) {
        db.update(authSessions)
          .set({ lastSeenAt: now })
          .where(
            and(
              eq(authSessions.id, record.sessionId),
              lt(authSessions.lastSeenAt, refreshThreshold)
            )
          )
          .catch(() => {});
      }
    }
  } catch {
    // 忽略异常，未命中有效 session 时保持 context.user 为空
  }
});
