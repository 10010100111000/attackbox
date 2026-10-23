import { eq, and, asc, desc, isNull, sql } from 'drizzle-orm'
import { db } from '../db'
import { modules, moduleRevisions, chapters, chapterRevisions, moduleRevisionChapters, chapterQuestions, chapterRevisionAssets, auditLogs, contentCategories, assets } from '../db/schema'
import { fail, slugFor, validateModule, questionSnapshot, validateOrder } from './content-input'
import type { ModuleInput, ChapterInput, AdminModuleDetail, AdminRevision } from '../../shared/content'
type Tx = Parameters<Parameters<typeof db.transaction>[0]>[0]

async function lockModule(tx: Tx, id: string) {
  const [mod] = await tx.select().from(modules).where(and(eq(modules.id, id), isNull(modules.archivedAt))).for('update')
  if (!mod) fail('模块不存在或已删除', 404)
  return mod
}
async function activeRevision(tx: Tx, mod: typeof modules.$inferSelect) {
  const [draft] = await tx.select().from(moduleRevisions).where(and(eq(moduleRevisions.moduleId, mod.id), eq(moduleRevisions.state, 'draft')))
  if (draft) return draft
  if (mod.publishedRevisionId) {
    const [published] = await tx.select().from(moduleRevisions).where(eq(moduleRevisions.id, mod.publishedRevisionId))
    if (published) return published
  }
  const [history] = await tx.select().from(moduleRevisions).where(and(eq(moduleRevisions.moduleId, mod.id), eq(moduleRevisions.state, 'published'))).orderBy(desc(moduleRevisions.revisionNo)).limit(1)
  return history
}
function checkVersion(rev: typeof moduleRevisions.$inferSelect | undefined, input: ModuleInput) {
  if (input.baseRevisionId !== undefined && input.baseRevisionId !== (rev?.id ?? null)) fail('草稿版本已变化，请刷新后重试', 409)
  if (input.lockVersion !== undefined && input.lockVersion !== (rev?.state === 'draft' ? rev.lockVersion : 0)) fail('内容已被其他管理员修改，请刷新后重试', 409)
}
// All mutation routes lock the stable module first and use this single fork path.
async function ensureDraft(tx: Tx, mod: typeof modules.$inferSelect, actor: string) {
  const source = await activeRevision(tx, mod)
  if (source?.state === 'draft') return source
  const [last] = await tx.select().from(moduleRevisions).where(eq(moduleRevisions.moduleId, mod.id)).orderBy(desc(moduleRevisions.revisionNo)).limit(1)
  const [draft] = await tx.insert(moduleRevisions).values({
    moduleId: mod.id, revisionNo: (last?.revisionNo || 0) + 1, state: 'draft',
    title: source?.title || '未命名模块', summary: source?.summary,
    descriptionMarkdown: source?.descriptionMarkdown, difficulty: source?.difficulty,
    categoryId: source?.categoryId, coverAssetId: source?.coverAssetId, createdBy: actor,
  }).returning()
  if (source) {
    const links = await tx.select().from(moduleRevisionChapters).where(eq(moduleRevisionChapters.moduleRevisionId, source.id))
    if (links.length) await tx.insert(moduleRevisionChapters).values(links.map(link => ({ ...link, moduleRevisionId: draft.id })))
  }
  return draft
}
async function bump(tx: Tx, id: string) {
  const [rev] = await tx.select().from(moduleRevisions).where(eq(moduleRevisions.id, id))
  const [updated] = await tx.update(moduleRevisions).set({ lockVersion: rev.lockVersion + 1, updatedAt: new Date() }).where(eq(moduleRevisions.id, id)).returning()
  return updated
}
async function audit(tx: Tx, actor: string, id: string, action: string) {
  await tx.insert(auditLogs).values({ actorUserId: actor, action, targetType: 'module', targetId: id })
}
async function writeChapter(tx: Tx, mod: typeof modules.$inferSelect, draft: typeof moduleRevisions.$inferSelect, actor: string, input: ChapterInput) {
  if (!input.title?.trim() || input.title.length > 160) fail('章节标题不能为空，且不能超过 160 字')
  if (input.bodyMarkdown !== undefined && typeof input.bodyMarkdown !== 'string') fail('章节正文必须是文本')
  if (!['public', 'authenticated'].includes(input.access)) fail('章节访问权限无效')
  let chapterId = input.chapterId
  let oldRevisionId: string | undefined
  if (chapterId) {
    const [link] = await tx.select().from(moduleRevisionChapters).where(and(eq(moduleRevisionChapters.moduleRevisionId, draft.id), eq(moduleRevisionChapters.chapterId, chapterId)))
    if (!link) fail('章节不属于当前草稿', 409)
    oldRevisionId = link.chapterRevisionId
  } else {
    const slug = slugFor(input.slug, 'chapter')
    const [taken] = await tx.select().from(chapters).where(and(eq(chapters.moduleId, mod.id), eq(chapters.slug, slug)))
    if (taken) fail('章节访问标识已存在', 409)
    const [chapter] = await tx.insert(chapters).values({ moduleId: mod.id, slug }).returning()
    chapterId = chapter.id
  }
  const oldQuestions = oldRevisionId ? await tx.select().from(chapterQuestions).where(eq(chapterQuestions.chapterRevisionId, oldRevisionId)).orderBy(asc(chapterQuestions.position)) : []
  const questionInputs = input.questions === undefined ? oldQuestions.map(q => ({ ...q, answer: undefined })) : input.questions
  if (!Array.isArray(questionInputs)) fail('题目列表无效')
  const seen = new Set<string>()
  const snapshots = questionInputs.map((q, index) => {
    if (!q || typeof q !== 'object') fail('题目格式无效')
    const old = oldQuestions.find(old => (q.questionKey && old.questionKey === q.questionKey) || (q.id && old.id === q.id))
    if (old && q.id && q.id !== old.id) fail('题目标识与版本不一致，请刷新', 409)
    if ((q.id || q.questionKey) && !old && oldRevisionId) fail('题目版本已变化或不属于该章节，请刷新', 409)
    const result = questionSnapshot(q, old)
    if (seen.has(result.questionKey)) fail('题目标识不能重复')
    seen.add(result.questionKey)
    return { ...result, position: index + 1 }
  })
  const [oldRevision] = oldRevisionId ? await tx.select().from(chapterRevisions).where(eq(chapterRevisions.id, oldRevisionId)) : []
  const [revision] = await tx.insert(chapterRevisions).values({
    chapterId: chapterId!, title: input.title.trim(), access: input.access,
    bodyMarkdown: input.bodyMarkdown ?? oldRevision?.bodyMarkdown ?? '',
    sourceKind: oldRevision?.sourceKind || 'editor', sourceUrl: oldRevision?.sourceUrl,
    sourceCommit: oldRevision?.sourceCommit, sourceSha256: oldRevision?.sourceSha256,
    createdBy: actor, isSealed: false,
  }).returning()
  if (snapshots.length) await tx.insert(chapterQuestions).values(snapshots.map(q => ({ ...q, chapterRevisionId: revision.id })))
  if (oldRevisionId) {
    const attachments = await tx.select().from(chapterRevisionAssets).where(eq(chapterRevisionAssets.chapterRevisionId, oldRevisionId))
    if (attachments.length) await tx.insert(chapterRevisionAssets).values(attachments.map(a => ({ ...a, chapterRevisionId: revision.id })))
    await tx.update(moduleRevisionChapters).set({ chapterRevisionId: revision.id }).where(and(eq(moduleRevisionChapters.moduleRevisionId, draft.id), eq(moduleRevisionChapters.chapterId, chapterId!)))
  } else {
    const links = await tx.select().from(moduleRevisionChapters).where(eq(moduleRevisionChapters.moduleRevisionId, draft.id))
    await tx.insert(moduleRevisionChapters).values({ moduleRevisionId: draft.id, moduleId: mod.id, chapterId: chapterId!, chapterRevisionId: revision.id, position: Math.max(0, ...links.map(link => link.position)) + 1 })
  }
  return { chapterId: chapterId!, revision }
}
async function reorder(tx: Tx, revisionId: string, ids: string[]) {
  const links = await tx.select().from(moduleRevisionChapters).where(eq(moduleRevisionChapters.moduleRevisionId, revisionId))
  validateOrder(ids, links.map(l => l.chapterId))
  const offset = Math.max(0, ...links.map(l => l.position)) + ids.length + 1
  for (let i = 0; i < ids.length; i++) await tx.update(moduleRevisionChapters).set({ position: offset + i }).where(and(eq(moduleRevisionChapters.moduleRevisionId, revisionId), eq(moduleRevisionChapters.chapterId, ids[i])))
  for (let i = 0; i < ids.length; i++) await tx.update(moduleRevisionChapters).set({ position: i + 1 }).where(and(eq(moduleRevisionChapters.moduleRevisionId, revisionId), eq(moduleRevisionChapters.chapterId, ids[i])))
}
async function metadata(tx: Tx, draftId: string, input: ModuleInput) {
  validateModule(input)
  if (input.categoryId) {
    const [category] = await tx.select().from(contentCategories).where(eq(contentCategories.id, input.categoryId))
    if (!category) fail('分类不存在')
  }
  const update: Partial<typeof moduleRevisions.$inferInsert> = {}
  if (input.coverAssetId) {
    const [cover] = await tx.select().from(assets).where(eq(assets.id, input.coverAssetId))
    if (!cover || cover.visibility !== 'public') fail('封面必须使用公开资源')
  }
  for (const key of ['title', 'summary', 'difficulty', 'descriptionMarkdown', 'categoryId', 'coverAssetId'] as const) if (input[key] !== undefined) (update as any)[key] = input[key]
  if (Object.keys(update).length) await tx.update(moduleRevisions).set(update).where(eq(moduleRevisions.id, draftId))
}
export async function saveModule(id: string | null, actor: string, input: ModuleInput) {
  validateModule(input)
  if (!id && !input.title?.trim()) fail('请填写模块名称')
  return db.transaction(async tx => {
    let mod: typeof modules.$inferSelect
    if (id) mod = await lockModule(tx, id)
    else {
      const slug = slugFor(input.slug, 'module')
      const [taken] = await tx.select().from(modules).where(eq(modules.slug, slug))
      if (taken) fail('模块访问标识已存在', 409)
      ;[mod] = await tx.insert(modules).values({ slug, createdBy: actor }).returning()
    }
    checkVersion(await activeRevision(tx, mod), input)
    const draft = await ensureDraft(tx, mod, actor)
    await metadata(tx, draft.id, input)
    if (input.chapters !== undefined) {
      if (!Array.isArray(input.chapters)) fail('章节列表无效')
      const ids: string[] = []
      for (const chapter of input.chapters) ids.push((await writeChapter(tx, mod, draft, actor, chapter)).chapterId)
      if (new Set(ids).size !== ids.length) fail('章节不能重复')
      const links = await tx.select().from(moduleRevisionChapters).where(eq(moduleRevisionChapters.moduleRevisionId, draft.id))
      for (const link of links) if (!ids.includes(link.chapterId)) await tx.delete(moduleRevisionChapters).where(and(eq(moduleRevisionChapters.moduleRevisionId, draft.id), eq(moduleRevisionChapters.chapterId, link.chapterId)))
      await reorder(tx, draft.id, ids)
    }
    const updated = id ? await bump(tx, draft.id) : (await tx.select().from(moduleRevisions).where(eq(moduleRevisions.id, draft.id)))[0]
    await audit(tx, actor, mod.id, id ? 'admin.module.save' : 'admin.module.create')
    return { module: mod, draft: updated }
  })
}
export async function addChapter(id: string, actor: string, input: ChapterInput & ModuleInput) {
  return db.transaction(async tx => {
    const mod = await lockModule(tx, id)
    checkVersion(await activeRevision(tx, mod), input)
    const draft = await ensureDraft(tx, mod, actor)
    const result = await writeChapter(tx, mod, draft, actor, input)
    const updated = await bump(tx, draft.id)
    await audit(tx, actor, id, 'admin.chapter.create')
    const [link] = await tx.select().from(moduleRevisionChapters).where(and(eq(moduleRevisionChapters.moduleRevisionId, draft.id), eq(moduleRevisionChapters.chapterId, result.chapterId)))
    return { chapter: { id: result.chapterId }, revision: result.revision, position: link.position, draft: updated }
  })
}
export async function saveChapter(id: string, actor: string, input: ChapterInput & ModuleInput & { moduleId: string }) {
  return db.transaction(async tx => {
    const mod = await lockModule(tx, input.moduleId)
    checkVersion(await activeRevision(tx, mod), input)
    const draft = await ensureDraft(tx, mod, actor)
    const result = await writeChapter(tx, mod, draft, actor, { ...input, chapterId: id })
    const updated = await bump(tx, draft.id)
    await audit(tx, actor, mod.id, 'admin.chapter.save')
    const questions = await tx.select({ id: chapterQuestions.id, questionKey: chapterQuestions.questionKey, position: chapterQuestions.position, promptMarkdown: chapterQuestions.promptMarkdown, errorHint: chapterQuestions.errorHint, normalization: chapterQuestions.normalization, hasAnswer: sql<boolean>`${chapterQuestions.answerHmac} is not null` }).from(chapterQuestions).where(eq(chapterQuestions.chapterRevisionId, result.revision.id)).orderBy(asc(chapterQuestions.position))
    return { revision: result.revision, questions, draft: updated }
  })
}
export async function reorderChapters(id: string, actor: string, input: ModuleInput & { orderedChapterIds?: string[]; orders?: { chapterId: string; position: number }[] }) {
  return db.transaction(async tx => {
    const mod = await lockModule(tx, id)
    checkVersion(await activeRevision(tx, mod), input)
    const draft = await ensureDraft(tx, mod, actor)
    if (input.orders && !input.orderedChapterIds) {
      const positions = input.orders.map(order => order.position)
      if (new Set(positions).size !== positions.length || positions.some(position => !Number.isInteger(position) || position < 1 || position > positions.length)) fail('排序位置必须连续且不能重复')
    }
    const ids = input.orderedChapterIds || input.orders?.slice().sort((a,b) => a.position-b.position).map(o => o.chapterId)
    if (!ids) fail('缺少章节顺序')
    await reorder(tx, draft.id, ids)
    const updated = await bump(tx, draft.id)
    await audit(tx, actor, id, 'admin.chapter.reorder')
    return updated
  })
}
export async function lifecycle(id: string, actor: string, action: 'publish' | 'unpublish' | 'discard' | 'delete', input: ModuleInput = {}) {
  return db.transaction(async tx => {
    const mod = await lockModule(tx, id)
    const active = await activeRevision(tx, mod)
    checkVersion(active, input)
    if (action === 'publish') {
      if (!active || active.state !== 'draft') fail('没有待发布草稿')
      if (!active.title?.trim()) fail('模块名称不能为空')
      const links = await tx.select().from(moduleRevisionChapters).where(eq(moduleRevisionChapters.moduleRevisionId, active.id))
      if (!links.length) fail('至少添加一个章节后再发布')
      for (const link of links) {
        const [chapter] = await tx.select().from(chapterRevisions).where(eq(chapterRevisions.id, link.chapterRevisionId))
        if (!chapter?.title.trim() || !chapter.bodyMarkdown.trim()) fail('所有章节需要标题和正文')
        const questions = await tx.select().from(chapterQuestions).where(eq(chapterQuestions.chapterRevisionId, link.chapterRevisionId))
        if (questions.some(q => !q.promptMarkdown.trim() || !q.answerHmac)) fail('存在未设置题目或标准答案的练习')
        if (!chapter.isSealed) await tx.update(chapterRevisions).set({ isSealed: true }).where(eq(chapterRevisions.id, chapter.id))
      }
      await tx.update(moduleRevisions).set({ state: 'published', publishedAt: new Date() }).where(eq(moduleRevisions.id, active.id))
      await tx.update(modules).set({ publishedRevisionId: active.id }).where(eq(modules.id, id))
    } else if (action === 'unpublish') {
      if (mod.publishedRevisionId) await ensureDraft(tx, mod, actor)
      await tx.update(modules).set({ publishedRevisionId: null }).where(eq(modules.id, id))
    } else if (action === 'discard') {
      if (active?.state !== 'draft') fail('当前没有可移除的草稿')
      const [history] = await tx.select().from(moduleRevisions).where(and(eq(moduleRevisions.moduleId, id), eq(moduleRevisions.state, 'published'))).limit(1)
      if (!history) fail('该模块尚未发布，请使用删除模块')
      await tx.delete(moduleRevisionChapters).where(eq(moduleRevisionChapters.moduleRevisionId, active.id))
      await tx.delete(moduleRevisions).where(eq(moduleRevisions.id, active.id))
    } else {
      // Archive rather than destroy revisions and future learning evidence.
      // 同时释放 slug(加 -archived-<id8> 后缀),避免被删除模块永久占用访问标识。
      const suffix = `-archived-${id.slice(0, 8)}`
      const base = mod.slug.slice(0, Math.max(1, 100 - suffix.length)).replace(/-+$/, '') || 'module'
      await tx.update(modules).set({ archivedAt: new Date(), publishedRevisionId: null, slug: `${base}${suffix}` }).where(eq(modules.id, id))
    }
    await audit(tx, actor, id, `admin.module.${action}`)
    if (action === 'publish' || action === 'unpublish') {
      const [module] = await tx.select().from(modules).where(eq(modules.id, id))
      if (action === 'unpublish') return module
      const [publishedRevision] = await tx.select().from(moduleRevisions).where(eq(moduleRevisions.id, module.publishedRevisionId!))
      return { module, publishedRevision }
    }
    return { id }
  })
}
export async function getModuleDetail(id: string): Promise<AdminModuleDetail> {
  const [mod] = await db.select().from(modules).where(and(eq(modules.id, id), isNull(modules.archivedAt)))
  if (!mod) fail('模块不存在或已删除', 404)
  const revisions = await db.select().from(moduleRevisions).where(eq(moduleRevisions.moduleId, id)).orderBy(desc(moduleRevisions.revisionNo))
  const draft = revisions.find(r => r.state === 'draft') || null
  const published = revisions.find(r => r.id === mod.publishedRevisionId) || null
  const source = draft || published || revisions.find(r => r.state === 'published') || null
  const links = source ? await db.select().from(moduleRevisionChapters).where(eq(moduleRevisionChapters.moduleRevisionId, source.id)).orderBy(asc(moduleRevisionChapters.position)) : []
  const chapterList = []
  for (const link of links) {
    const [chapter] = await db.select().from(chapters).where(eq(chapters.id, link.chapterId))
    const [revision] = await db.select().from(chapterRevisions).where(eq(chapterRevisions.id, link.chapterRevisionId))
    const qs = await db.select().from(chapterQuestions).where(eq(chapterQuestions.chapterRevisionId, revision.id)).orderBy(asc(chapterQuestions.position))
    chapterList.push({ chapterId: chapter.id, slug: chapter.slug, chapterSlug: chapter.slug, position: link.position,
      chapterRevisionId: revision.id, draftRevisionId: revision.id, title: revision.title, access: revision.access,
      bodyMarkdown: revision.bodyMarkdown, isSealed: revision.isSealed,
      questions: qs.map(q => ({ id: q.id, questionKey: q.questionKey, position: q.position, promptMarkdown: q.promptMarkdown,
        errorHint: q.errorHint, normalization: q.normalization, hasAnswer: q.answerHmac !== null })) })
  }
  const [category] = source?.categoryId ? await db.select().from(contentCategories).where(eq(contentCategories.id, source.categoryId)) : []
  const toRevision = (revision: typeof moduleRevisions.$inferSelect | null): AdminRevision | null => revision ? { ...revision, updatedAt: revision.updatedAt.toISOString(), publishedAt: revision.publishedAt?.toISOString() || null } : null
  return { module: { ...mod, createdAt: mod.createdAt.toISOString(), archivedAt: mod.archivedAt?.toISOString() || null }, draft: toRevision(draft), published: toRevision(published), editable: toRevision(source), category: category || null,
    isPublished: Boolean(mod.publishedRevisionId), hasDraft: Boolean(draft), chapterCount: links.length,
    baseRevisionId: source?.id || null, lockVersion: draft?.lockVersion || 0, chapters: chapterList }
}
