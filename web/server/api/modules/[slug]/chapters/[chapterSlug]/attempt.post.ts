import { defineEventHandler, getRouterParam, readBody, createError } from 'h3'
import { db } from '../../../../../db'
import {
  modules,
  moduleRevisionChapters,
  chapters,
  chapterRevisions,
  chapterQuestions,
} from '../../../../../db/schema'
import { eq, and, isNotNull, isNull } from 'drizzle-orm'
import { verifyQuizAnswer } from '../../../../../utils/quiz'

/**
 * 章节练习题判题:仅对当前已发布版本中的题目做常量时间 HMAC 比对,
 * 只返回对错与错误提示,绝不回传标准答案或 answer_hmac。
 */
export default defineEventHandler(async (event) => {
  const moduleSlug = getRouterParam(event, 'slug')
  const chapterSlug = getRouterParam(event, 'chapterSlug')
  const currentUser = event.context.user
  const body = (await readBody(event)) || {}

  if (!moduleSlug || !chapterSlug) {
    throw createError({ statusCode: 400, message: '缺少模块或章节标识' })
  }

  const questionId = typeof body.questionId === 'string' ? body.questionId : ''
  const answer = typeof body.answer === 'string' ? body.answer : ''
  if (!questionId) {
    throw createError({ statusCode: 400, message: '缺少题目 ID' })
  }
  if (answer.length > 2000) {
    throw createError({ statusCode: 400, message: '答案长度超出限制' })
  }

  const [mod] = await db
    .select({ id: modules.id, publishedRevisionId: modules.publishedRevisionId })
    .from(modules)
    .where(
      and(
        eq(modules.slug, moduleSlug),
        isNotNull(modules.publishedRevisionId),
        isNull(modules.archivedAt)
      )
    )
    .limit(1)

  if (!mod || !mod.publishedRevisionId) {
    throw createError({ statusCode: 404, message: '模块不存在或尚未公开发布' })
  }

  const [targetChapter] = await db
    .select({
      chapterRevisionId: chapterRevisions.id,
      access: chapterRevisions.access,
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
    .limit(1)

  if (!targetChapter) {
    throw createError({ statusCode: 404, message: '该章节不存在或未包含在当前发布版本中' })
  }

  if (targetChapter.access === 'authenticated') {
    if (!currentUser || currentUser.status !== 'active') {
      throw createError({ statusCode: 401, message: '该章节为受限学习内容,请登录账号后继续学习' })
    }
  }

  const [question] = await db
    .select({
      id: chapterQuestions.id,
      answerHmac: chapterQuestions.answerHmac,
      normalization: chapterQuestions.normalization,
      errorHint: chapterQuestions.errorHint,
    })
    .from(chapterQuestions)
    .where(
      and(
        eq(chapterQuestions.id, questionId),
        eq(chapterQuestions.chapterRevisionId, targetChapter.chapterRevisionId)
      )
    )
    .limit(1)

  if (!question) {
    throw createError({ statusCode: 404, message: '题目不存在或不属于本章节' })
  }

  const correct = question.answerHmac
    ? verifyQuizAnswer(answer, question.answerHmac, question.normalization)
    : false

  return {
    ok: true,
    data: {
      correct,
      errorHint: correct ? null : question.errorHint,
    },
  }
})
