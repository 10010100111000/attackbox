import { defineEventHandler, getRouterParam, createError } from 'h3';
import { db } from '../../../../db';
import {
  modules,
  moduleRevisions,
  moduleRevisionChapters,
  chapters,
  chapterRevisions,
  chapterQuestions,
} from '../../../../db/schema';
import { eq, and, asc, isNotNull } from 'drizzle-orm';
import { requireAdmin } from '../../../../utils/admin';

export default defineEventHandler(async (event) => {
  requireAdmin(event);
  const moduleId = getRouterParam(event, 'id');

  if (!moduleId) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '缺少模块 ID',
    });
  }

  const [mod] = await db
    .select()
    .from(modules)
    .where(eq(modules.id, moduleId))
    .limit(1);

  if (!mod) {
    throw createError({
      statusCode: 404,
      statusMessage: 'Not Found',
      message: '模块不存在',
    });
  }

  // 获取当前草稿
  const [draft] = await db
    .select()
    .from(moduleRevisions)
    .where(
      and(
        eq(moduleRevisions.moduleId, moduleId),
        eq(moduleRevisions.state, 'draft')
      )
    )
    .limit(1);

  // 获取当前已发布版本
  let published = null;
  if (mod.publishedRevisionId) {
    const [pub] = await db
      .select()
      .from(moduleRevisions)
      .where(eq(moduleRevisions.id, mod.publishedRevisionId))
      .limit(1);
    published = pub || null;
  }

  // 优先展示当前草稿章节，草稿不存在时展示已发布版本的章节清单
  const activeRev = draft || published;
  let activeChapters = [];
  if (activeRev) {
    const mrcList = await db
      .select({
        position: moduleRevisionChapters.position,
        chapterId: chapters.id,
        chapterSlug: chapters.slug,
        chapterRevisionId: chapterRevisions.id,
        title: chapterRevisions.title,
        access: chapterRevisions.access,
        bodyMarkdown: chapterRevisions.bodyMarkdown,
        isSealed: chapterRevisions.isSealed,
        createdAt: chapterRevisions.createdAt,
      })
      .from(moduleRevisionChapters)
      .innerJoin(chapters, eq(moduleRevisionChapters.chapterId, chapters.id))
      .innerJoin(chapterRevisions, eq(moduleRevisionChapters.chapterRevisionId, chapterRevisions.id))
      .where(eq(moduleRevisionChapters.moduleRevisionId, activeRev.id))
      .orderBy(asc(moduleRevisionChapters.position));

    for (const item of mrcList) {
      // 查询该章节修订下的题目列表（白名单返回，不暴露 raw answer_hmac）
      const questions = await db
        .select({
          id: chapterQuestions.id,
          questionKey: chapterQuestions.questionKey,
          position: chapterQuestions.position,
          promptMarkdown: chapterQuestions.promptMarkdown,
          errorHint: chapterQuestions.errorHint,
          normalization: chapterQuestions.normalization,
          answerKeyVersion: chapterQuestions.answerKeyVersion,
          hasAnswer: isNotNull(chapterQuestions.answerHmac),
        })
        .from(chapterQuestions)
        .where(eq(chapterQuestions.chapterRevisionId, item.chapterRevisionId))
        .orderBy(asc(chapterQuestions.position));

      activeChapters.push({
        ...item,
        questions,
      });
    }
  }

  return {
    ok: true,
    data: {
      module: mod,
      draft: draft || null,
      published,
      chapters: activeChapters,
    },
  };
});
