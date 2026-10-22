import { defineEventHandler } from 'h3';
import { db } from '../../db';
import { modules, moduleRevisions, moduleRevisionChapters } from '../../db/schema';
import { eq, isNotNull, isNull, and, desc, sql } from 'drizzle-orm';

export default defineEventHandler(async () => {
  // 查询所有已发布且未归档的模块
  const publishedModules = await db
    .select({
      id: modules.id,
      slug: modules.slug,
      publishedRevisionId: modules.publishedRevisionId,
      createdAt: modules.createdAt,
      title: moduleRevisions.title,
      summary: moduleRevisions.summary,
      difficulty: moduleRevisions.difficulty,
      coverAssetId: moduleRevisions.coverAssetId,
      publishedAt: moduleRevisions.publishedAt,
    })
    .from(modules)
    .innerJoin(moduleRevisions, eq(modules.publishedRevisionId, moduleRevisions.id))
    .where(
      and(
        isNotNull(modules.publishedRevisionId),
        isNull(modules.archivedAt),
        eq(moduleRevisions.state, 'published')
      )
    )
    .orderBy(desc(moduleRevisions.publishedAt));

  const items = [];

  for (const mod of publishedModules) {
    // 统计当前发布版本章节数
    const [cnt] = await db
      .select({ count: sql<number>`count(*)` })
      .from(moduleRevisionChapters)
      .where(eq(moduleRevisionChapters.moduleRevisionId, mod.publishedRevisionId!));

    items.push({
      id: mod.id,
      slug: mod.slug,
      title: mod.title,
      summary: mod.summary,
      difficulty: mod.difficulty,
      coverAssetId: mod.coverAssetId,
      publishedAt: mod.publishedAt,
      chapterCount: Number(cnt?.count || 0),
    });
  }

  return {
    ok: true,
    data: items,
  };
});
