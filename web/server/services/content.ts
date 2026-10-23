import { eq, and, asc, sql, inArray } from 'drizzle-orm'
import { db } from '../db'
import { modules, chapters, chapterQuestions, auditLogs, contentCategories, assets, modulePrerequisites } from '../db/schema'
import { fail, slugFor, validateModule, questionValues } from './content-input'
import type { ModuleInput, ChapterInput, AdminModuleDetail } from '../../shared/content'
type Tx = Parameters<Parameters<typeof db.transaction>[0]>[0]

async function lockModule(tx: Tx, id: string, input: ModuleInput) {
  const [mod] = await tx.select().from(modules).where(eq(modules.id, id)).for('update')
  if (!mod) fail('模块不存在', 404)
  if (input.lockVersion === undefined) fail('缺少修改标识，请刷新后重试', 409)
  if (input.lockVersion !== mod.lockVersion) fail('内容已被其他管理员修改，请刷新后重试', 409)
  return mod
}
async function audit(tx: Tx, actor: string, id: string, action: string) {
  await tx.insert(auditLogs).values({ actorUserId: actor, action, targetType: 'module', targetId: id })
}
async function validateReferences(tx: Tx, input: ModuleInput) {
  if (input.categoryId) {
    const [category] = await tx.select().from(contentCategories).where(eq(contentCategories.id, input.categoryId))
    if (!category) fail('分类不存在')
  }
  if (input.coverAssetId) {
    const [cover] = await tx.select().from(assets).where(eq(assets.id, input.coverAssetId))
    if (!cover || cover.visibility !== 'public') fail('封面必须使用公开资源')
  }
}
async function writeChapter(tx: Tx, moduleId: string, input: ChapterInput, position: number, existing: typeof chapters.$inferSelect[]) {
  if (!input || typeof input !== 'object') fail('章节格式无效')
  if (typeof input.title !== 'string' || !input.title.trim() || input.title.length > 160) fail('章节标题不能为空，且不能超过 160 字')
  if (typeof input.bodyMarkdown !== 'string') fail('章节正文必须是文本')
  if (!['public', 'authenticated'].includes(input.access)) fail('章节访问权限无效')
  const old = existing.find(c => c.id === input.chapterId)
  if (input.chapterId && !old) fail('章节不属于当前模块', 409)
  if (input.durationMinutes !== undefined && input.durationMinutes !== null && (!Number.isInteger(input.durationMinutes) || input.durationMinutes < 1 || input.durationMinutes > 1440)) fail('预计时长需为 1–1440 分钟的整数，或留空')
  const values = { title: input.title.trim(), access: input.access, bodyMarkdown: input.bodyMarkdown, position, durationMinutes: input.durationMinutes === undefined ? old?.durationMinutes ?? null : input.durationMinutes }
  const [chapter] = old
    ? await tx.update(chapters).set(values).where(eq(chapters.id, old.id)).returning()
    : await tx.insert(chapters).values({ ...values, moduleId, slug: slugFor(input.slug, 'chapter') }).returning()
  if (input.questions !== undefined) {
    if (!Array.isArray(input.questions)) fail('题目列表无效')
    const oldQuestions = await tx.select().from(chapterQuestions).where(eq(chapterQuestions.chapterId, chapter.id))
    // Vacate positions before moving retained questions. Their IDs and answers stay stable.
    const offset = Math.max(0, ...oldQuestions.map(q => q.position)) + input.questions.length + 1
    await tx.update(chapterQuestions).set({ position: sql`${chapterQuestions.position} + ${offset}` }).where(eq(chapterQuestions.chapterId, chapter.id))
    const retained = new Set<string>()
    for (const [index, q] of input.questions.entries()) {
      if (!q || typeof q !== 'object') fail('题目格式无效')
      const previous = oldQuestions.find(old => old.id === q.id)
      if (q.id && !previous) fail('题目不属于该章节，请刷新', 409)
      if (q.id && retained.has(q.id)) fail('题目不能重复')
      const values = { ...questionValues(q, previous), position: index + 1 }
      const [saved] = previous
        ? await tx.update(chapterQuestions).set(values).where(eq(chapterQuestions.id, previous.id)).returning()
        : await tx.insert(chapterQuestions).values({ ...values, chapterId: chapter.id }).returning()
      retained.add(saved.id)
    }
    for (const old of oldQuestions) if (!retained.has(old.id)) await tx.delete(chapterQuestions).where(eq(chapterQuestions.id, old.id))
  }
  return chapter.id
}
async function validatePublication(tx: Tx, id: string) {
  const list = await tx.select().from(chapters).where(eq(chapters.moduleId, id))
  if (!list.length) fail('至少添加一个章节后再发布')
  for (const ch of list) {
    if (!ch.title.trim() || !ch.bodyMarkdown.trim()) fail('所有章节需要标题和正文')
    const questions = await tx.select().from(chapterQuestions).where(eq(chapterQuestions.chapterId, ch.id))
    if (questions.some(q => !q.promptMarkdown.trim() || !q.answerHmac)) fail('存在未设置题目或标准答案的练习')
  }
}
async function writePrerequisites(tx: Tx, moduleId: string, ids: string[]) {
  ids = ids.map(id => id.toLowerCase())
  if (ids.includes(moduleId)) fail('模块不能将自身设为前置模块')
  if (ids.length) {
    const found = await tx.select({ id: modules.id }).from(modules).where(inArray(modules.id, ids))
    if (found.length !== ids.length) fail('前置模块不存在，请重新选择')
  }
  await tx.delete(modulePrerequisites).where(eq(modulePrerequisites.moduleId, moduleId))
  if (!ids.length) return
  await tx.insert(modulePrerequisites).values(ids.map((prerequisiteId, index) => ({ moduleId, prerequisiteId, position: index + 1 })))
  const cycle = await tx.execute(sql`
    with recursive reachable(id) as (
      select prerequisite_id from module_prerequisites where module_id = ${moduleId}::uuid
      union
      select p.prerequisite_id from module_prerequisites p join reachable r on p.module_id = r.id
    ) select id from reachable where id = ${moduleId}::uuid limit 1
  `)
  if (cycle.length) fail('前置模块形成循环，请调整学习顺序')
}
export async function saveModule(id: string | null, actor: string, input: ModuleInput) {
  validateModule(input)
  if (!id && !input.title?.trim()) fail('请填写模块名称')
  return db.transaction(async tx => {
    // Order graph writes before module row locks, including deletes, to avoid cross-module deadlocks.
    await tx.execute(sql`select pg_advisory_xact_lock(72486211)`)
    await validateReferences(tx, input)
    const metadata: Partial<typeof modules.$inferInsert> = {}
    for (const key of ['title', 'summary', 'descriptionMarkdown', 'difficulty', 'categoryId', 'coverAssetId', 'learningOutcomes'] as const)
      if (input[key] !== undefined) (metadata as any)[key] = input[key]
    if (metadata.title) metadata.title = metadata.title.trim()
    if (metadata.learningOutcomes) metadata.learningOutcomes = metadata.learningOutcomes.map(item => item.trim())
    let mod: typeof modules.$inferSelect
    if (id) {
      mod = await lockModule(tx, id, input)
      if (input.slug !== undefined && slugFor(input.slug, 'module') !== mod.slug) fail('创建后不能修改访问标识')
      ;[mod] = await tx.update(modules).set({ ...metadata, lockVersion: mod.lockVersion + 1, updatedAt: new Date() }).where(eq(modules.id, id)).returning()
    } else {
      const slug = slugFor(input.slug, 'module')
      ;[mod] = await tx.insert(modules).values({ ...metadata, title: input.title!.trim(), slug, createdBy: actor }).returning()
    }
    if (input.chapters !== undefined) {
      if (!Array.isArray(input.chapters)) fail('章节列表无效')
      const existing = await tx.select().from(chapters).where(eq(chapters.moduleId, mod.id))
      const supplied = input.chapters.map(c => c?.chapterId).filter(Boolean)
      if (new Set(supplied).size !== supplied.length) fail('章节不能重复')
      // Delete removed records first to free their slugs; all changes roll back on failure.
      for (const old of existing) if (!supplied.includes(old.id)) await tx.delete(chapters).where(eq(chapters.id, old.id))
      const offset = Math.max(0, ...existing.map(c => c.position)) + input.chapters.length + 1
      await tx.update(chapters).set({ position: sql`${chapters.position} + ${offset}` }).where(eq(chapters.moduleId, mod.id))
      for (const [index, ch] of input.chapters.entries()) await writeChapter(tx, mod.id, ch, index + 1, existing)
    }
    if (input.prerequisiteModuleIds !== undefined) await writePrerequisites(tx, mod.id, input.prerequisiteModuleIds)
    const status = input.intent === 'draft' ? 'unpublished' : input.intent === 'publish' ? 'published' : mod.status
    if (status === 'published') await validatePublication(tx, mod.id)
    if (input.intent) {
      ;[mod] = await tx.update(modules).set({ status, publishedAt: status === 'published' ? (mod.publishedAt || new Date()) : mod.publishedAt }).where(eq(modules.id, mod.id)).returning()
    }
    await audit(tx, actor, mod.id, id ? 'admin.module.save' : 'admin.module.create')
    return { module: mod }
  }).catch(error => {
    if (error?.code === '23505' || error?.cause?.code === '23505') fail('模块或章节访问标识已存在', 409)
    throw error
  })
}
export async function lifecycle(id: string, actor: string, action: 'publish' | 'unpublish' | 'delete', input: ModuleInput) {
  validateModule(input)
  return db.transaction(async tx => {
    await tx.execute(sql`select pg_advisory_xact_lock(72486211)`)
    const mod = await lockModule(tx, id, input)
    if (action === 'delete') await tx.delete(modules).where(eq(modules.id, id))
    else {
      if (action === 'publish') await validatePublication(tx, id)
      await tx.update(modules).set({ status: action === 'publish' ? 'published' : 'unpublished',
        publishedAt: action === 'publish' ? (mod.publishedAt || new Date()) : mod.publishedAt,
        lockVersion: mod.lockVersion + 1, updatedAt: new Date() }).where(eq(modules.id, id))
    }
    await audit(tx, actor, id, `admin.module.${action}`)
    return { id }
  })
}
export async function getModuleDetail(id: string): Promise<AdminModuleDetail> {
  return db.transaction(async tx => {
    const [mod] = await tx.select().from(modules).where(eq(modules.id, id))
    if (!mod) fail('模块不存在', 404)
    const list = await tx.select().from(chapters).where(eq(chapters.moduleId, id)).orderBy(asc(chapters.position))
    const chapterList = []
    for (const ch of list) {
      const qs = await tx.select({ id: chapterQuestions.id, position: chapterQuestions.position,
        promptMarkdown: chapterQuestions.promptMarkdown, errorHint: chapterQuestions.errorHint,
        answer: chapterQuestions.answer, normalization: chapterQuestions.normalization, hasAnswer: sql<boolean>`${chapterQuestions.answerHmac} is not null`,
      }).from(chapterQuestions).where(eq(chapterQuestions.chapterId, ch.id)).orderBy(asc(chapterQuestions.position))
      chapterList.push({ chapterId: ch.id, slug: ch.slug, position: ch.position, title: ch.title, access: ch.access, bodyMarkdown: ch.bodyMarkdown, durationMinutes: ch.durationMinutes, questions: qs })
    }
    const [category] = mod.categoryId ? await tx.select().from(contentCategories).where(eq(contentCategories.id, mod.categoryId)) : []
    const prerequisites = await tx.select({ id: modules.id, slug: modules.slug, title: modules.title, status: modules.status }).from(modulePrerequisites).innerJoin(modules, eq(modulePrerequisites.prerequisiteId, modules.id)).where(eq(modulePrerequisites.moduleId, id)).orderBy(asc(modulePrerequisites.position))
    return { prerequisites, module: { ...mod, createdAt: mod.createdAt.toISOString(), updatedAt: mod.updatedAt.toISOString(), publishedAt: mod.publishedAt?.toISOString() || null }, category: category || null, chapters: chapterList }
  }, { isolationLevel: 'repeatable read', accessMode: 'read only' })
}
