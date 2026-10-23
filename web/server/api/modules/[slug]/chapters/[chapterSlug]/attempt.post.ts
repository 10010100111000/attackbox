import { defineEventHandler, getRouterParam, readBody, createError } from 'h3'
import { db } from '../../../../../db'
import { modules, chapters, chapterQuestions } from '../../../../../db/schema'
import { eq, and } from 'drizzle-orm'
import { verifyQuizAnswer } from '../../../../../utils/quiz'
export default defineEventHandler(async event => {
  const body = (await readBody(event)) || {}
  const questionId = typeof body.questionId === 'string' ? body.questionId : ''
  const answer = typeof body.answer === 'string' ? body.answer : ''
  if (!/^[0-9a-f-]{36}$/i.test(questionId) || answer.length > 2000)
    throw createError({ statusCode: 400, message: '无效题目标识或答案过长' })
  const [question] = await db.select({ access: chapters.access, answerHmac: chapterQuestions.answerHmac,
    normalization: chapterQuestions.normalization, errorHint: chapterQuestions.errorHint,
  }).from(chapterQuestions).innerJoin(chapters, eq(chapterQuestions.chapterId, chapters.id))
    .innerJoin(modules, eq(chapters.moduleId, modules.id)).where(and(
      eq(modules.slug, getRouterParam(event, 'slug') || ''), eq(modules.status, 'published'),
      eq(chapters.slug, getRouterParam(event, 'chapterSlug') || ''), eq(chapterQuestions.id, questionId),
    ))
  if (!question) throw createError({ statusCode: 404, message: '题目不存在或模块尚未发布，请刷新页面' })
  if (question.access === 'authenticated' && event.context.user?.status !== 'active')
    throw createError({ statusCode: 401, message: '请登录后提交答案' })
  const correct = question.answerHmac ? verifyQuizAnswer(answer, question.answerHmac, question.normalization) : false
  return { ok: true, data: { correct, errorHint: correct ? null : question.errorHint } }
})
