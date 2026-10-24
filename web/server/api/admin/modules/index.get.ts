import { defineEventHandler } from 'h3'
import { eq, desc, sql } from 'drizzle-orm'
import { db } from '../../../db'
import { modules, chapters, contentCategories } from '../../../db/schema'
import { requireAdmin } from '../../../utils/admin'
export default defineEventHandler(async event => {
  requireAdmin(event)
  const records = await db.select({ id: modules.id, slug: modules.slug, title: modules.title,
    summary: modules.summary, defaultTargetTemplateId:modules.defaultTargetTemplateId, difficulty: modules.difficulty, categoryId: modules.categoryId,
    coverAssetId: modules.coverAssetId, status: modules.status, lockVersion: modules.lockVersion,
    createdAt: modules.createdAt, updatedAt: modules.updatedAt, publishedAt: modules.publishedAt,
    chapterCount: sql<number>`(select count(*)::int from ${chapters} where ${chapters.moduleId} = ${modules.id})`,
    category: { id: contentCategories.id, slug: contentCategories.slug, name: contentCategories.name },
  }).from(modules).leftJoin(contentCategories, eq(modules.categoryId, contentCategories.id)).orderBy(desc(modules.updatedAt))
  return { ok: true, data: records }
})
