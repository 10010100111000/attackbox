import { eq, inArray, sql } from 'drizzle-orm'
import type { db } from '../db'
import { auditLogs, modulePrerequisites, modules, targetTemplates } from '../db/schema'
import { fail, validateTemplateId } from './content-input'

export type Tx = Parameters<Parameters<typeof db.transaction>[0]>[0]
export type ChapterTargetMode = 'inherit' | 'none' | 'custom'
export const chapterTargetModes: readonly ChapterTargetMode[] = ['inherit', 'none', 'custom']

export async function auditModule(tx: Tx, actor: string, id: string, action: string) {
  await tx
    .insert(auditLogs)
    .values({ actorUserId: actor, action, targetType: 'module', targetId: id })
}

/** 锁定模块行并比对修改标识（乐观锁），未命中返回 404，版本冲突或缺失返回 409。 */
export async function lockModule(tx: Tx, id: string, lockVersion: number | undefined) {
  const [mod] = await tx.select().from(modules).where(eq(modules.id, id)).for('update')
  if (!mod) fail('模块不存在', 404)
  if (lockVersion === undefined) fail('缺少修改标识，请刷新后重试', 409)
  if (lockVersion !== mod.lockVersion) fail('内容已被其他管理员修改，请刷新后重试', 409)
  return mod
}

/** 靶机模板必须存在且已启用；沿用原有引用时允许模板已被停用。 */
export async function checkTargetTemplate(tx: Tx, id: string | null, previous: string | null) {
  if (!id) return
  const [template] = await tx
    .select({ id: targetTemplates.id, enabled: targetTemplates.enabled })
    .from(targetTemplates)
    .where(eq(targetTemplates.id, id))
    .for('share')
  if (!template) fail('靶机模板不存在，请重新选择')
  if (!template.enabled && id.toLowerCase() !== previous?.toLowerCase())
    fail('该靶机模板已停用，请选择已启用模板')
}

/**
 * 章节靶机配置的唯一解析入口：合并旧值与变更、校验组合规则与模板可用性。
 * 完整保存（writeChapter）与运行配置保存共用，避免规则漂移。
 */
export async function resolveChapterTarget(
  tx: Tx,
  old: { targetMode: string; targetTemplateId: string | null } | undefined,
  input: { targetMode?: string; targetTemplateId?: string | null }
) {
  const targetMode = (input.targetMode ?? old?.targetMode ?? 'inherit') as ChapterTargetMode
  if (!chapterTargetModes.includes(targetMode)) fail('章节实验环境选项无效')
  if (input.targetTemplateId !== undefined) validateTemplateId(input.targetTemplateId)
  const targetTemplateId =
    targetMode === 'custom'
      ? input.targetTemplateId === undefined
        ? (old?.targetTemplateId ?? null)
        : input.targetTemplateId
      : null
  if (targetMode !== 'custom' && input.targetTemplateId) fail('仅另选模板时可以指定章节靶机')
  if (targetMode === 'custom' && !targetTemplateId) fail('请为章节选择靶机模板')
  await checkTargetTemplate(
    tx,
    targetTemplateId,
    old?.targetMode === 'custom' ? old.targetTemplateId : null
  )
  return { targetMode, targetTemplateId }
}

export async function writePrerequisites(tx: Tx, moduleId: string, ids: string[]) {
  ids = ids.map(id => id.toLowerCase())
  if (ids.includes(moduleId)) fail('模块不能将自身设为前置模块')
  if (ids.length) {
    const found = await tx.select({ id: modules.id }).from(modules).where(inArray(modules.id, ids))
    if (found.length !== ids.length) fail('前置模块不存在，请重新选择')
  }
  await tx.delete(modulePrerequisites).where(eq(modulePrerequisites.moduleId, moduleId))
  if (!ids.length) return
  await tx
    .insert(modulePrerequisites)
    .values(ids.map((prerequisiteId, index) => ({ moduleId, prerequisiteId, position: index + 1 })))
  const cycle = await tx.execute(sql`
  with recursive reachable(id) as (
   select prerequisite_id from module_prerequisites where module_id = ${moduleId}::uuid
   union
   select p.prerequisite_id from module_prerequisites p join reachable r on p.module_id = r.id
  ) select id from reachable where id = ${moduleId}::uuid limit 1
 `)
  if (cycle.length) fail('前置模块形成循环，请调整学习顺序')
}
