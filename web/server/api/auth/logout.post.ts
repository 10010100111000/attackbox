import { defineEventHandler } from 'h3'
import { db } from '../../db'
import { authSessions } from '../../db/schema'
import { eq } from 'drizzle-orm'
import { clearSessionCookie } from '../../utils/auth'

export default defineEventHandler(async event => {
  const session = event.context.session

  if (session?.id) {
    await db
      .update(authSessions)
      .set({ revokedAt: new Date() })
      .where(eq(authSessions.id, session.id))
  }

  clearSessionCookie(event)

  return {
    message: '已安全退出登录'
  }
})
