import { defineEventHandler, createError } from 'h3'
import { and, eq, ne } from 'drizzle-orm'
import { db } from '../../db'
import { targetInstances } from '../../db/schema'
import { sandboxController } from '../../sandbox'
import { sandboxAction, requireSandboxUser, sandboxIdentity } from '../../utils/sandbox'

export default defineEventHandler(event => sandboxAction(async () => {
  const userId = requireSandboxUser(event)
  const controller = sandboxController()
  if (!controller.status(userId).available) {
    throw createError({ statusCode: 503, message: '练习环境尚未开放' })
  }
  const [activeTarget] = await db
    .select({ nodeId: targetInstances.nodeId })
    .from(targetInstances)
    .where(and(eq(targetInstances.userId, userId), ne(targetInstances.state, 'stopped')))
    .limit(1)
  return {
    ok: true,
    data: await controller.start(userId, await sandboxIdentity(userId), activeTarget?.nodeId ?? null)
  }
}))
