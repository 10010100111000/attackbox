import { eq, desc, sql } from 'drizzle-orm'
import { db } from '../db'
import { modules, chapters, contentCategories } from '../db/schema'
export async function listPublishedModules(limit?: number) {
  const query = db.select({ id: modules.id, slug: modules.slug, title: modules.title,
    summary: modules.summary, difficulty: modules.difficulty, categoryId: modules.categoryId,
    coverAssetId: modules.coverAssetId, publishedAt: modules.publishedAt, updatedAt: modules.updatedAt,
    durationMinutes: sql<number>`(select coalesce(sum(duration_minutes),0)::int from ${chapters} where ${chapters.moduleId} = ${modules.id})`,
    category: { id: contentCategories.id, slug: contentCategories.slug, name: contentCategories.name },
    chapterCount: sql<number>`(select count(*)::int from ${chapters} where ${chapters.moduleId} = ${modules.id})`,
  }).from(modules).leftJoin(contentCategories, eq(modules.categoryId, contentCategories.id))
    .where(eq(modules.status, 'published')).orderBy(desc(modules.publishedAt), desc(modules.id)).$dynamic()
  return limit === undefined ? query : query.limit(limit)
}
