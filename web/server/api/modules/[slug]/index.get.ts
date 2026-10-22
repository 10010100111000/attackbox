import { defineEventHandler, getRouterParam, createError } from 'h3';
import { db } from '../../../db';
import {
  modules,
  moduleRevisions,
  moduleRevisionChapters,
  chapters,
  chapterRevisions,
  chapterQuestions,
} from '../../../db/schema';
import { eq, and, isNotNull, isNull, asc, sql } from 'drizzle-orm';

export default defineEventHandler(async (event) => {
  const slug = getRouterParam(event, 'slug');
  const currentUser = event.context.user;

  if (!slug) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '缺少模块 Slug',
    });
  }

  // 1. 查询当前已发布模块
  const [mod] = await db
    .select({
      id: modules.id,
      slug: modules.slug,
      publishedRevisionId: modules.publishedRevisionId,
      createdAt: modules.createdAt,
    })
    .from(modules)
    .where(
      and(
        eq(modules.slug, slug),
        isNotNull(modules.publishedRevisionId),
        isNull(modules.archivedAt)
      )
    )
    .limit(1);

  if (!mod || !mod.publishedRevisionId) {
    throw createError({
      statusCode: 404,
      statusMessage: 'Not Found',
      message: '模块不存在或尚未公开发布',
    });
  }

  // 2. 查询当前发布版本
  const [pubRev] = await db
    .select({
      id: moduleRevisions.id,
      revisionNo: moduleRevisions.revisionNo,
      title: moduleRevisions.title,
      summary: moduleRevisions.summary,
      descriptionMarkdown: moduleRevisions.descriptionMarkdown,
      difficulty: moduleRevisions.difficulty,
      coverAssetId: moduleRevisions.coverAssetId,
      publishedAt: moduleRevisions.publishedAt,
    })
    .from(moduleRevisions)
    .where(eq(moduleRevisions.id, mod.publishedRevisionId))
    .limit(1);

  // 3. 查询已发布章节清单
  const chapterList = await db
    .select({
      chapterId: chapters.id,
      slug: chapters.slug,
      title: chapterRevisions.title,
      access: chapterRevisions.access,
      position: moduleRevisionChapters.position,
      chapterRevisionId: chapterRevisions.id,
    })
    .from(moduleRevisionChapters)
    .innerJoin(chapters, eq(moduleRevisionChapters.chapterId, chapters.id))
    .innerJoin(chapterRevisions, eq(moduleRevisionChapters.chapterRevisionId, chapterRevisions.id))
    .where(eq(moduleRevisionChapters.moduleRevisionId, mod.publishedRevisionId))
    .orderBy(asc(moduleRevisionChapters.position));

  const chaptersWithMeta = [];
  for (const ch of chapterList) {
    // 统计题目数
    const [qCnt] = await db
      .select({ count: sql<number>`count(*)` })
      .from(chapterQuestions)
      .where(eq(chapterQuestions.chapterRevisionId, ch.chapterRevisionId));

    const isLocked = ch.access === 'authenticated' && !currentUser;

    chaptersWithMeta.push({
      id: ch.chapterId,
      slug: ch.slug,
      title: ch.title,
      access: ch.access,
      position: ch.position,
      questionsCount: Number(qCnt?.count || 0),
      isLocked,
    });
  }

  return {
    ok: true,
    data: {
      module: {
        id: mod.id,
        slug: mod.slug,
        title: pubRev.title,
        summary: pubRev.summary,
        descriptionMarkdown: pubRev.descriptionMarkdown,
        difficulty: pubRev.difficulty,
        coverAssetId: pubRev.coverAssetId,
        publishedAt: pubRev.publishedAt,
      },
      chapters: chaptersWithMeta,
    },
  };
});
