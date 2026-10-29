import { eq, sql } from 'drizzle-orm'
import { db } from '../db'
import { chapters, modules, moduleSources } from '../db/schema'
import { fail, validateModule } from './content-input'
import {
  auditModule,
  checkTargetTemplate,
  lockModule,
  resolveChapterTarget,
  writePrerequisites
} from './module-references'

import type { ModuleInput } from '../../shared/content'

const RUNTIME_FIELDS = [
  'lockVersion',
  'defaultTargetTemplateId',
  'prerequisiteModuleIds',
  'chapters'
]
const CHAPTER_FIELDS = ['chapterId', 'targetMode', 'targetTemplateId']
type ChapterChange = { chapterId: string; targetMode?: string; targetTemplateId?: string | null }

/** Git 管理模块的运行配置：只触碰靶机与前置关系，不读取或改写教程内容。 */
export async function saveModuleRuntime(id: string, actor: string, body: Record<string, unknown>) {
  if (!body || typeof body !== 'object' || Array.isArray(body)) fail('运行配置必须是对象')
  if (Object.keys(body).some(k => !RUNTIME_FIELDS.includes(k))) fail('运行配置接口不能修改教程内容')
  const input: Partial<ModuleInput> = {
    lockVersion:
      typeof body.lockVersion === 'number' ? body.lockVersion : (body.lockVersion as any),
    defaultTargetTemplateId:
      body.defaultTargetTemplateId === null || typeof body.defaultTargetTemplateId === 'string'
        ? body.defaultTargetTemplateId
        : (body.defaultTargetTemplateId as any),
    prerequisiteModuleIds: Array.isArray(body.prerequisiteModuleIds)
      ? body.prerequisiteModuleIds
      : (body.prerequisiteModuleIds as any)
  }
  // lockVersion、模板标识和前置模块的格式规则与完整保存共用 validateModule。
  validateModule(input)
  const changes = (body.chapters ?? []) as ChapterChange[]
  if (
    !Array.isArray(changes) ||
    changes.some(
      c =>
        !c ||
        typeof c !== 'object' ||
        typeof c.chapterId !== 'string' ||
        Object.keys(c).some(k => !CHAPTER_FIELDS.includes(k))
    ) ||
    new Set(changes.map(c => c.chapterId)).size !== changes.length
  )
    fail('章节运行配置无效')
  const { lockVersion, defaultTargetTemplateId, prerequisiteModuleIds } = body as {
    lockVersion?: number
    defaultTargetTemplateId?: string | null
    prerequisiteModuleIds?: string[]
  }
  if (lockVersion === undefined) fail('缺少修改标识，请刷新后重试', 409)

  return db.transaction(async tx => {
    await tx.execute(sql`select pg_advisory_xact_lock(72486211)`)
    const [source] = await tx.select().from(moduleSources).where(eq(moduleSources.moduleId, id))
    if (!source) fail('该模块尚未接管到 Git 管理', 409)
    const mod = await lockModule(tx, id, lockVersion)

    if (defaultTargetTemplateId !== undefined)
      await checkTargetTemplate(tx, defaultTargetTemplateId, mod.defaultTargetTemplateId)
    if (changes.length) {
      const existing = new Map(
        (
          await tx
            .select({
              id: chapters.id,
              targetMode: chapters.targetMode,
              targetTemplateId: chapters.targetTemplateId
            })
            .from(chapters)
            .where(eq(chapters.moduleId, id))
        ).map(c => [c.id, c])
      )
      for (const change of changes) {
        const old = existing.get(change.chapterId)
        if (!old) fail('章节不属于当前模块', 409)
        const target = await resolveChapterTarget(tx, old, change)
        await tx.update(chapters).set(target).where(eq(chapters.id, old.id))
      }
    }
    if (prerequisiteModuleIds !== undefined)
      await writePrerequisites(tx, mod.id, prerequisiteModuleIds)
    const [saved] = await tx
      .update(modules)
      .set({
        ...(defaultTargetTemplateId !== undefined ? { defaultTargetTemplateId } : {}),
        lockVersion: mod.lockVersion + 1,
        updatedAt: new Date()
      })
      .where(eq(modules.id, mod.id))
      .returning()
    await auditModule(tx, actor, mod.id, 'admin.module.save')
    return { module: saved }
  })
}
