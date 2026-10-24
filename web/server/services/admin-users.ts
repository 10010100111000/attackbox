import { and, eq, gt, isNull, sql } from 'drizzle-orm'
import { createError } from 'h3'
import { db } from '../db'
import { users, authSessions, auditLogs } from '../db/schema'
import { sandboxController } from '../sandbox'
import { learningHistory } from './learning'

const fields = { id: users.id, email: users.email, displayName: users.displayName, role: users.role, status: users.status, createdAt: users.createdAt, lastLoginAt: users.lastLoginAt, emailVerifiedAt: users.emailVerifiedAt }
export function userId(value: unknown) {
  if (typeof value !== 'string' || !/^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i.test(value)) throw createError({ statusCode: 400, message: '无效用户标识' })
  return value
}
export async function listUsers(query: Record<string, unknown>) {
  const search = typeof query.search === 'string' ? query.search.trim().slice(0, 100) : ''
  const status = query.status || '', role = query.role || ''
  if (status && !['active', 'pending', 'suspended'].includes(String(status))) throw createError({ statusCode: 400, message: '无效账号状态' })
  if (role && !['user', 'admin'].includes(String(role))) throw createError({ statusCode: 400, message: '无效账号角色' })
  const page = Number(query.page || 1)
  if (!Number.isSafeInteger(page) || page < 1 || page > 100000) throw createError({ statusCode: 400, message: '无效页码' })
  const where = and(search ? sql`(strpos(lower(${users.email}), lower(${search})) > 0 or strpos(lower(${users.displayName}), lower(${search})) > 0)` : undefined, status ? eq(users.status, String(status)) : undefined, role ? eq(users.role, String(role)) : undefined)
  return db.transaction(async tx => {
    const [total] = await tx.select({ count: sql<number>`count(*)::int` }).from(users).where(where)
    const items = await tx.select(fields).from(users).where(where).orderBy(sql`${users.createdAt} desc`, users.id).limit(20).offset((page - 1) * 20)
    return { items: items.map(u => ({ ...u, instance: sandboxController().status(u.id).instance })), total: total!.count, page, pageSize: 20 }
  }, { isolationLevel: 'repeatable read', accessMode: 'read only' })
}
export async function userDetail(id: string) {
  const [user] = await db.select(fields).from(users).where(eq(users.id, id))
  if (!user) throw createError({ statusCode: 404, message: '用户不存在' })
  const [sessions] = await db.select({ count: sql<number>`count(*)::int` }).from(authSessions).where(and(eq(authSessions.userId, id), isNull(authSessions.revokedAt), gt(authSessions.expiresAt, new Date())))
  const [progress] = await db.execute<{ modules: number; visited: number; completed: number }>(sql`
    select count(distinct c.module_id)::int as modules, count(*)::int as visited,
      count(p.completed_at)::int as completed from chapter_progress p
      join chapters c on c.id=p.chapter_id join modules m on m.id=c.module_id
      where p.user_id=${id} and m.status='published'`)
  return { user, activeSessions: sessions!.count, progress, history: await learningHistory(id), instance: sandboxController().status(id).instance }
}
// Serialize status changes through the DB commit and the in-memory scheduler update.
let mutationTail: Promise<unknown> = Promise.resolve()
export function changeUserStatus(actorId: string, id: string, suspended: boolean) {
  const task = mutationTail.then(async () => {
    const status = await db.transaction(async tx => {
      await tx.execute(sql`select pg_advisory_xact_lock(61290347)`)
      const [actor] = await tx.select(fields).from(users).where(eq(users.id, actorId)).for('update')
      if (actor?.role !== 'admin' || actor.status !== 'active') throw createError({ statusCode: 403, message: '管理员账号已不可用' })
      const [target] = await tx.select(fields).from(users).where(eq(users.id, id)).for('update')
      if (!target) throw createError({ statusCode: 404, message: '用户不存在' })
      if (suspended && id === actorId) throw createError({ statusCode: 409, message: '不能停用当前管理员账号' })
      if (suspended && target.role === 'admin' && target.status === 'active') {
        const [count] = await tx.select({ count: sql<number>`count(*)::int` }).from(users).where(and(eq(users.role, 'admin'), eq(users.status, 'active')))
        if (count!.count <= 1) throw createError({ statusCode: 409, message: '至少保留一个可用管理员' })
      }
      const status = suspended ? 'suspended' : target.emailVerifiedAt ? 'active' : 'pending'
      await tx.update(users).set({ status, updatedAt: new Date() }).where(eq(users.id, id))
      if (suspended) await tx.update(authSessions).set({ revokedAt: new Date() }).where(and(eq(authSessions.userId, id), isNull(authSessions.revokedAt)))
      await tx.insert(auditLogs).values({ actorUserId: actorId, action: suspended ? 'user.suspend' : 'user.restore', targetType: 'user', targetId: id, metadata: { previousStatus: target.status, status } })
      return status
    })
    const controller = sandboxController()
    controller.setUserBlocked(id, suspended)
    const instance = controller.status(id).instance
    // Start cleanup while serialized, but don't make every other account mutation
    // wait for a disconnected Worker to time out.
    const cleanup = suspended && instance
      ? controller.stopAdmin(instance.id).then(() => 'released' as const, () => 'pending' as const)
      : Promise.resolve('none' as const)
    return { status, cleanup }
  })
  mutationTail = task.catch(() => {})
  return task.then(async result => ({ status: result.status, cleanup: await result.cleanup }))
}

export async function revokeUserSessions(actorId: string, id: string) {
  await db.transaction(async tx => {
    await tx.execute(sql`select pg_advisory_xact_lock(61290347)`)
    const [actor] = await tx.select(fields).from(users).where(eq(users.id, actorId)).for('update')
    if (actor?.role !== 'admin' || actor.status !== 'active') throw createError({ statusCode: 403, message: '管理员账号已不可用' })
    const [target] = await tx.select({ id: users.id }).from(users).where(eq(users.id, id)).for('update')
    if (!target) throw createError({ statusCode: 404, message: '用户不存在' })
    await tx.update(authSessions).set({ revokedAt: new Date() }).where(and(eq(authSessions.userId, id), isNull(authSessions.revokedAt)))
    await tx.insert(auditLogs).values({ actorUserId: actorId, action: 'user.revoke_sessions', targetType: 'user', targetId: id })
  })
}
