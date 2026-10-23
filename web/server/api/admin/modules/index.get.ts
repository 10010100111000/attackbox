import { defineEventHandler } from 'h3'
import { eq, isNull, desc, inArray, sql } from 'drizzle-orm'
import { db } from '../../../db'
import { modules, moduleRevisions, moduleRevisionChapters, contentCategories } from '../../../db/schema'
import { requireAdmin } from '../../../utils/admin'
export default defineEventHandler(async event => {
  requireAdmin(event)
  const records = await db.select().from(modules).where(isNull(modules.archivedAt)).orderBy(desc(modules.createdAt))
  if (!records.length) return { ok: true, data: [] }
  // List DTO never loads article bodies or answers. Counts come from the selected revision.
  const revisions = await db.select({ id: moduleRevisions.id, moduleId: moduleRevisions.moduleId,
    revisionNo: moduleRevisions.revisionNo, state: moduleRevisions.state, title: moduleRevisions.title,
    summary: moduleRevisions.summary, difficulty: moduleRevisions.difficulty, categoryId: moduleRevisions.categoryId,
    lockVersion: moduleRevisions.lockVersion, updatedAt: moduleRevisions.updatedAt, publishedAt: moduleRevisions.publishedAt,
  }).from(moduleRevisions).where(inArray(moduleRevisions.moduleId, records.map(m => m.id))).orderBy(desc(moduleRevisions.revisionNo))
  const counts = revisions.length ? await db.select({ revisionId: moduleRevisionChapters.moduleRevisionId, count: sql<number>`count(*)::int` })
    .from(moduleRevisionChapters).where(inArray(moduleRevisionChapters.moduleRevisionId, revisions.map(r => r.id))).groupBy(moduleRevisionChapters.moduleRevisionId) : []
  const countMap = new Map(counts.map(c => [c.revisionId, Number(c.count)]))
  const categories = await db.select().from(contentCategories)
  return { ok: true, data: records.map(module => {
    const history = revisions.filter(r => r.moduleId === module.id)
    const draft = history.find(r => r.state === 'draft') || null
    const published = history.find(r => r.id === module.publishedRevisionId) || null
    const editable = draft || published || history.find(r => r.state === 'published') || null
    return { ...module, draft: draft ? { ...draft, chapterCount: countMap.get(draft.id) || 0 } : null,
      published, isPublished: Boolean(published), hasDraft: Boolean(draft),
      chapterCount: editable ? countMap.get(editable.id) || 0 : 0,
      publishedChapterCount: published ? countMap.get(published.id) || 0 : 0,
      category: categories.find(c => c.id === editable?.categoryId) || null }
  }) }
})
