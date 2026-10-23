import { createError, type H3Event } from 'h3'
import { eq } from 'drizzle-orm'
import { db } from '../db'
import { sandboxIdentities } from '../db/schema'
import { SandboxError } from '../sandbox/controller'
export function requireSandboxUser(event: H3Event) {
  if (event.context.user?.status !== 'active') throw createError({ statusCode: 401, message: '请先登录后使用练习环境' })
  return event.context.user.id
}
export async function sandboxIdentity(userId: string) {
  // Concurrent initial starts converge to the same durable numeric identity.
  await db.insert(sandboxIdentities).values({ userId }).onConflictDoNothing()
  const [identity] = await db.select().from(sandboxIdentities).where(eq(sandboxIdentities.userId, userId))
  if (!identity || !Number.isSafeInteger(identity.workerUserId)) throw createError({ statusCode: 500, message: '无法分配沙箱身份' })
  return identity.workerUserId
}
export function sandboxId(value: unknown) {
  if (typeof value !== 'string' || !/^sbx_[a-f0-9]{24}$/.test(value)) throw createError({ statusCode: 400, message: '无效实例标识' })
  return value
}

export async function sandboxAction<T>(action: () => T | Promise<T>): Promise<T> {
  try { return await action() }
  catch (error) {
    if (error instanceof SandboxError) throw createError({ statusCode: error.statusCode, message: error.message })
    throw error
  }
}
