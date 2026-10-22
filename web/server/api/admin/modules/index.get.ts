import { defineEventHandler } from 'h3';
import { db } from '../../../db';
import { modules, moduleRevisions, moduleRevisionChapters } from '../../../db/schema';
import { eq, desc, and, sql } from 'drizzle-orm';
import { requireAdmin } from '../../../utils/admin';

export default defineEventHandler(async (event) => {
  requireAdmin(event);

  // 查询所有模块
  const allModules = await db
    .select({
      id: modules.id,
      slug: modules.slug,
      publishedRevisionId: modules.publishedRevisionId,
      createdAt: modules.createdAt,
      archivedAt: modules.archivedAt,
    })
    .from(modules)
    .orderBy(desc(modules.createdAt));

  const items = [];

  for (const mod of allModules) {
    // 查询当前草稿
    const [draft] = await db
      .select({
        id: moduleRevisions.id,
        revisionNo: moduleRevisions.revisionNo,
        title: moduleRevisions.title,
        summary: moduleRevisions.summary,
        difficulty: moduleRevisions.difficulty,
        lockVersion: moduleRevisions.lockVersion,
        updatedAt: moduleRevisions.updatedAt,
      })
      .from(moduleRevisions)
      .where(
        and(
          eq(moduleRevisions.moduleId, mod.id),
          eq(moduleRevisions.state, 'draft')
        )
      )
      .limit(1);

    // 查询当前已发布版本
    let published = null;
    if (mod.publishedRevisionId) {
      const [pub] = await db
        .select({
          id: moduleRevisions.id,
          revisionNo: moduleRevisions.revisionNo,
          title: moduleRevisions.title,
          summary: moduleRevisions.summary,
          difficulty: moduleRevisions.difficulty,
          publishedAt: moduleRevisions.publishedAt,
        })
        .from(moduleRevisions)
        .where(eq(moduleRevisions.id, mod.publishedRevisionId))
        .limit(1);
      published = pub || null;
    }

    // 计算当前草稿包含的章节数
    let draftChapterCount = 0;
    if (draft) {
      const [cnt] = await db
        .select({ count: sql<number>`count(*)` })
        .from(moduleRevisionChapters)
        .where(eq(moduleRevisionChapters.moduleRevisionId, draft.id));
      draftChapterCount = Number(cnt?.count || 0);
    }

    items.push({
      ...mod,
      draft: draft ? { ...draft, chapterCount: draftChapterCount } : null,
      published,
    });
  }

  return {
    ok: true,
    data: items,
  };
});
