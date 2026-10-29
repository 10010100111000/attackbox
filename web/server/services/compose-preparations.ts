import { randomUUID } from 'node:crypto'
import { and, eq, sql } from 'drizzle-orm'
import { createError } from 'h3'
import { db } from '../db'
import { labPreparations, targetTemplates } from '../db/schema'
import { sandboxHub } from '../sandbox'
import { openVariables } from '../labs/credentials'
import { bundleRecord, uuid } from './compose-templates'
export type PreparationAction = 'status' | 'prepare' | 'cancel' | 'rebuild'
const active = (state: string) => ['preparing', 'cancelling', 'unknown'].includes(state)
// One statement gives a consistent template/revision/job snapshot without
// joining the mutation queue. Status RPC results still use attempt/version CAS.
async function readPreparation(id: string, nodeId: string) {
  const [snapshot] = await db
    .select({ t: targetTemplates, job: labPreparations })
    .from(targetTemplates)
    .leftJoin(
      labPreparations,
      and(
        eq(labPreparations.templateId, targetTemplates.id),
        eq(labPreparations.templateVersion, targetTemplates.revision),
        eq(labPreparations.nodeId, nodeId)
      )
    )
    .where(eq(targetTemplates.id, uuid(id)))
  if (!snapshot) throw createError({ statusCode: 404, message: 'Compose 模板不存在' })
  return { ...snapshot, dispatch: !!snapshot.job }
}
async function reservePreparation(
  id: string,
  nodeId: string,
  bootId: string,
  action: Exclude<PreparationAction, 'status'>,
  expectedAttemptId?: string
) {
  return db.transaction(async tx => {
    // Keep template revisions and preparation reservations ordered. Worker RPC
    // runs after this short transaction, outside the shared mutation lock.
    await tx.execute(sql`select pg_advisory_xact_lock(72486212)`)
    const [t] = await tx
      .select()
      .from(targetTemplates)
      .where(eq(targetTemplates.id, uuid(id)))
    if (!t) throw createError({ statusCode: 404, message: 'Compose 模板不存在' })
    const [old] = await tx
      .select()
      .from(labPreparations)
      .where(
        and(
          eq(labPreparations.templateId, t.id),
          eq(labPreparations.templateVersion, t.revision),
          eq(labPreparations.nodeId, nodeId)
        )
      )
    if (!old) {
      if (action !== 'prepare') return { t, job: null, dispatch: false }
      const [job] = await tx
        .insert(labPreparations)
        .values({
          templateId: t.id,
          templateVersion: t.revision,
          nodeId: nodeId,
          bootId: bootId,
          state: 'preparing'
        })
        .returning()
      return { t, job: job!, dispatch: true }
    }
    if (expectedAttemptId !== old.attemptId)
      throw createError({ statusCode: 409, message: '准备任务已变化，请重新读取节点准备状态' })
    if (action === 'cancel') {
      if (!active(old.state)) return { t, job: old, dispatch: false }
      const [job] = await tx
        .update(labPreparations)
        .set({ state: 'cancelling', version: old.version + 1, updatedAt: new Date() })
        .where(eq(labPreparations.id, old.id))
        .returning()
      return { t, job: job!, dispatch: true }
    }
    if (active(old.state)) return { t, job: old, dispatch: false }
    if (action === 'prepare' && old.state === 'ready') return { t, job: old, dispatch: false }
    const [job] = await tx
      .update(labPreparations)
      .set({
        attemptId: randomUUID(),
        mode: action === 'rebuild' || old.mode === 'refresh' ? 'refresh' : 'cached',
        state: 'preparing',
        logs: [],
        error: null,
        version: old.version + 1,
        updatedAt: new Date()
      })
      .where(eq(labPreparations.id, old.id))
      .returning()
    return { t, job: job!, dispatch: true }
  })
}
export async function preparationAction(
  id: string,
  nodeId: string,
  action: PreparationAction,
  expectedAttemptId?: string
) {
  const hub = sandboxHub(),
    node = hub.snapshot().nodes.find(n => n.id === nodeId && n.state === 'online' && n.bootId)
  if (!node || !hub.composeNodes.has(node.id))
    throw createError({
      statusCode: 409,
      message: '节点未启用当前版本 Compose 能力，请一起升级控制端与 Worker'
    })
  if (['prepare', 'rebuild'].includes(action) && node.settings.draining)
    throw createError({ statusCode: 409, message: '节点已暂停新任务' })
  const reservation =
    action === 'status'
      ? await readPreparation(id, node.id)
      : await reservePreparation(id, node.id, node.bootId!, action, expectedAttemptId)
  const { t, job } = reservation
  if (!job || !reservation.dispatch) return job
  const type =
    action === 'status'
      ? 'compose_prepare_status'
      : action === 'cancel'
        ? 'compose_prepare_cancel'
        : 'compose_prepare'
  let result
  try {
    const payload = {
      requested_at: new Date().toISOString(),
      target_id: job.attemptId,
      owner_id: t.id,
      preparation_id: job.attemptId,
      deployment_id: job.deploymentId || undefined
    }
    if (type === 'compose_prepare') {
      const b = await bundleRecord(t.bundleId!)
      Object.assign(payload, {
        bundle_id: b.id,
        bundle_hash: b.hash,
        settings: t.settings,
        variables: openVariables(t.id, t.variablesCipher),
        rebuild: job.mode === 'refresh'
      })
    }
    result = await hub.composeRpc(node.id, node.bootId!, type, payload, 15000)
  } catch {
    const [saved] = await db
      .update(labPreparations)
      .set({
        state: 'unknown',
        error: '节点结果未确认，请手动查询；重新准备前请先取消旧任务',
        version: job.version + 1,
        updatedAt: new Date()
      })
      .where(
        and(
          eq(labPreparations.id, job.id),
          eq(labPreparations.attemptId, job.attemptId),
          eq(labPreparations.version, job.version)
        )
      )
      .returning()
    return (
      saved || (await db.select().from(labPreparations).where(eq(labPreparations.id, job.id)))[0]!
    )
  }
  let state =
    result.success &&
    ['preparing', 'ready', 'failed', 'unknown', 'cancelling', 'cancelled'].includes(
      result.job_state || ''
    )
      ? result.job_state!
      : 'failed'
  if (action === 'cancel' && !result.success) state = 'unknown'
  const verified =
    result.success &&
    !!result.deployment_id &&
    (state === 'ready' || (state === 'unknown' && result.deployment_id === job.deploymentId))
  if (state === 'ready' && !verified) state = 'failed'
  if (state === 'unknown' && verified && job.state === 'ready') state = 'ready'
  const errors: Record<string, string> = {
    preparation_request_expired: '准备请求已过期，请检查控制端和节点时间后重试',
    compose_prepare_busy: '节点正在准备其他部署，请稍后重试',
    compose_prepare_failed: '准备失败，请检查配置和准备记录',
    compose_prepare_timeout: '准备超过 30 分钟，已终止，可重试',
    worker_disk_pressure: '节点磁盘空间不足，请先释放空间',
    compose_queue_full: '节点操作繁忙，请稍后重试'
  }
  const [saved] = await db
    .update(labPreparations)
    .set({
      state,
      ...(verified
        ? {
            bootId: node.bootId!,
            deploymentId: result.deployment_id,
            summary: result.summary || job.summary
          }
        : {}),
      logs: result.logs || job.logs,
      error: result.error
        ? errors[result.error] || '节点未完成准备，请检查配置后重试'
        : state === 'unknown'
          ? '节点无此任务，请取消后重新准备'
          : null,
      version: job.version + 1,
      updatedAt: new Date()
    })
    .where(
      and(
        eq(labPreparations.id, job.id),
        eq(labPreparations.attemptId, job.attemptId),
        eq(labPreparations.version, job.version)
      )
    )
    .returning()
  return (
    saved || (await db.select().from(labPreparations).where(eq(labPreparations.id, job.id)))[0]!
  )
}
