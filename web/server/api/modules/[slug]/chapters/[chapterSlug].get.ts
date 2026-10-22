import { defineEventHandler, getRouterParam, createError } from 'h3';
import { db } from '../../../../db';
import {
  modules,
  moduleRevisionChapters,
  chapters,
  chapterRevisions,
  chapterQuestions,
} from '../../../../db/schema';
import { eq, and, isNotNull, isNull, asc } from 'drizzle-orm';

export default defineEventHandler(async (event) => {
  const moduleSlug = getRouterParam(event, 'slug');
  const chapterSlug = getRouterParam(event, 'chapterSlug');
  const currentUser = event.context.user;

  if (!moduleSlug || !chapterSlug) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '缺少模块或章节标识',
    });
  }

  // 1. 查询已发布模块
  const [mod] = await db
    .select({
      id: modules.id,
      slug: modules.slug,
      publishedRevisionId: modules.publishedRevisionId,
    })
    .from(modules)
    .where(
      and(
        eq(modules.slug, moduleSlug),
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

  // 2. 查找该模块当前已发布版本中对应的章节
  const [targetChapter] = await db
    .select({
      chapterId: chapters.id,
      slug: chapters.slug,
      position: moduleRevisionChapters.position,
      chapterRevisionId: chapterRevisions.id,
      title: chapterRevisions.title,
      access: chapterRevisions.access,
      bodyMarkdown: chapterRevisions.bodyMarkdown,
      isSealed: chapterRevisions.isSealed,
    })
    .from(moduleRevisionChapters)
    .innerJoin(chapters, eq(moduleRevisionChapters.chapterId, chapters.id))
    .innerJoin(chapterRevisions, eq(moduleRevisionChapters.chapterRevisionId, chapterRevisions.id))
    .where(
      and(
        eq(moduleRevisionChapters.moduleRevisionId, mod.publishedRevisionId),
        eq(chapters.slug, chapterSlug)
      )
    )
    .limit(1);

  if (!targetChapter) {
    throw createError({
      statusCode: 404,
      statusMessage: 'Not Found',
      message: '该章节不存在或未包含在当前发布版本中',
    });
  }

  // 3. 访问权限检查：受限内容必须登录
  if (targetChapter.access === 'authenticated') {
    if (!currentUser || currentUser.status !== 'active') {
      throw createError({
        statusCode: 401,
        statusMessage: 'Unauthorized',
        message: '该章节为受限学习内容，请登录账号后继续学习',
      });
    }
  }

  // 4. 获取题目列表（白名单脱敏，绝不暴露 answerHmac）
  const questions = await db
    .select({
      id: chapterQuestions.id,
      position: chapterQuestions.position,
      promptMarkdown: chapterQuestions.promptMarkdown,
      errorHint: chapterQuestions.errorHint,
      normalization: chapterQuestions.normalization,
    })
    .from(chapterQuestions)
    .where(eq(chapterQuestions.chapterRevisionId, targetChapter.chapterRevisionId))
    .orderBy(asc(chapterQuestions.position));

  return {
    ok: true,
    data: {
      chapter: {
        id: targetChapter.chapterId,
        slug: targetChapter.slug,
        title: targetChapter.title,
        access: targetChapter.access,
        position: targetChapter.position,
        bodyMarkdown: targetChapter.bodyMarkdown,
      },
      questions,
    },
  };
});
