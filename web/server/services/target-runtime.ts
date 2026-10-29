import { getRuntimePolicy } from './runtime-policy'
import { experimentNode } from '../../shared/target-runtime'
import { randomBytes } from 'node:crypto'
import { and, eq, ne, sql } from 'drizzle-orm'
import { createError } from 'h3'
import { db } from '../db'
import {
  targetInstances,
  targetTemplates,
  labPreparations,
  chapters,
  modules,
  challenges
} from '../db/schema'
import { composeTargetOperation } from './compose-target-runtime'
import { chapterContext } from './learning'
import { sandboxHub, sandboxController } from '../sandbox'

export async function targetScope(
  user: string,
  input: Record<string, unknown>,
  runner: typeof db = db
) {
  if (input.kind === 'challenge') {
    if (typeof input.challengeId !== 'string' || !/^[a-f0-9-]{36}$/.test(input.challengeId)) {
      throw createError({ statusCode: 400, message: '挑战参数无效' })
    }
    const [c] = await runner
      .select()
      .from(challenges)
      .where(and(eq(challenges.id, input.challengeId), eq(challenges.status, 'published')))
    if (!c) {
      throw createError({ statusCode: 404, message: '挑战不可用' })
    }
    return { scope: 'challenge:' + c.id, templateId: c.targetTemplateId }
  }

  if (
    input.kind !== 'chapter' ||
    typeof input.moduleSlug !== 'string' ||
    typeof input.chapterSlug !== 'string'
  ) {
    throw createError({ statusCode: 400, message: '章节参数无效' })
  }
  const { chapter } = await chapterContext(runner, input.moduleSlug, input.chapterSlug, user)
  const [c] = await runner
    .select({
      mode: chapters.targetMode,
      template: chapters.targetTemplateId,
      moduleTemplate: modules.defaultTargetTemplateId,
      module: modules.id
    })
    .from(chapters)
    .innerJoin(modules, eq(modules.id, chapters.moduleId))
    .where(eq(chapters.id, chapter.id))

  return {
    scope: c!.mode === 'custom' ? 'chapter:' + chapter.id : 'module:' + c!.module,
    templateId: c!.mode === 'none' ? null : c!.mode === 'custom' ? c!.template : c!.moduleTemplate
  }
}

function view(row: typeof targetInstances.$inferSelect) {
  return {
    id: row.id,
    expiresAt: row.expiresAt,
    state: row.state,
    instance: row.result,
    updatedAt: row.updatedAt
  }
}

export async function targetOperation(
  user: string,
  input: Record<string, unknown>,
  action: 'start' | 'inspect' | 'stop'
) {
  const hub = sandboxHub()
  const scope = action === 'stop' ? null : await targetScope(user, input)

  if (action !== 'start') {
    const [existing] = await db
      .select()
      .from(targetInstances)
      .where(
        and(
          eq(targetInstances.userId, user),
          action === 'stop'
            ? eq(targetInstances.id, String(input.id || ''))
            : eq(targetInstances.scope, scope!.scope)
        )
      )
    if (!existing) return null
    if (existing.state === 'stopped') return view(existing)
    if (existing.expiresAt.getTime() <= Date.now()) action = 'stop'
    return composeTargetOperation(existing, action)
  }

  const row = await db.transaction(async tx => {
    await tx.execute(sql`select pg_advisory_xact_lock(19842, hashtext(${user}))`)
    const [existing] = await tx
      .select()
      .from(targetInstances)
      .where(and(eq(targetInstances.userId, user), eq(targetInstances.scope, scope!.scope)))

    if (!scope?.templateId) {
      throw createError({ statusCode: 409, message: '未关联靶机模板' })
    }
    const [template] = await tx
      .select()
      .from(targetTemplates)
      .where(and(eq(targetTemplates.id, scope.templateId), eq(targetTemplates.enabled, true)))
    if (!template) {
      throw createError({ statusCode: 409, message: '靶机模板已停用' })
    }
    const attacker = sandboxController().status(user).instance
    const activeTargets = await tx
      .select({ nodeId: targetInstances.nodeId })
      .from(targetInstances)
      .where(and(eq(targetInstances.userId, user), ne(targetInstances.state, 'stopped')))
    let preferredNodeId: string | null
    try {
      preferredNodeId = experimentNode(
        attacker?.nodeId,
        activeTargets.map(t => t.nodeId)
      )
    } catch {
      throw createError({
        statusCode: 409,
        message: '已有实验环境分布在不同节点，请先停止冲突的靶机'
      })
    }
    if (existing && existing.state !== 'stopped') {
      return existing
    }
    const nodes = hub
      .snapshot()
      .nodes.filter(
        n => n.state === 'online' && !n.settings.draining && n.bootId && hub.composeNodes.has(n.id)
      )
    const prepared = await tx
      .select()
      .from(labPreparations)
      .where(
        and(
          eq(labPreparations.templateId, template.id),
          eq(labPreparations.templateVersion, template.revision),
          sql`${labPreparations.deploymentId} is not null`
        )
      )
    const eligible = (preferredNodeId ? nodes.filter(n => n.id === preferredNodeId) : nodes).filter(
      n => prepared.some(p => p.nodeId === n.id && p.bootId === n.bootId && p.deploymentId)
    )

    if (!eligible.length) {
      throw createError({ statusCode: 503, message: '当前可用节点尚未准备此靶场，请联系管理员' })
    }

    const node = eligible[0]!
    const values = {
      id: 'tgt_' + randomBytes(16).toString('hex'),
      userId: user,
      scope: scope.scope,
      nodeId: node.id,
      bootId: node.bootId!,
      state: 'starting',
      expiresAt: new Date(Date.now() + (await getRuntimePolicy()).targetTtlMinutes * 60 * 1000),
      deploymentId: prepared.find(p => p.nodeId === node.id)!.deploymentId!,
      result: null,
      revision: (existing?.revision || 0) + 1,
      updatedAt: new Date()
    }
    if (existing) {
      await tx.delete(targetInstances).where(eq(targetInstances.id, existing.id))
    }
    const [created] = await tx.insert(targetInstances).values(values).returning()
    return created
  })

  if (!row) return null
  if (row.state === 'stopped') return view(row)
  if (row.expiresAt.getTime() <= Date.now()) action = 'stop'
  return composeTargetOperation(row, action)
}

export async function stopUserTargets(userId: string): Promise<'none' | 'released' | 'pending'> {
  const activeRows = await db
    .select()
    .from(targetInstances)
    .where(and(eq(targetInstances.userId, userId), ne(targetInstances.state, 'stopped')))

  if (!activeRows.length) return 'none'

  let allReleased = true
  for (const row of activeRows) {
    try {
      const res = await targetOperation(userId, { id: row.id }, 'stop')
      if (res?.state !== 'stopped') {
        allReleased = false
      }
    } catch {
      allReleased = false
    }
  }
  return allReleased ? 'released' : 'pending'
}
