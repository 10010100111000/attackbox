import { defineEventHandler, getRouterParam, readBody, createError } from 'h3'
import { db } from '../../../../../db'
import { chapterQuestions, questionCompletions } from '../../../../../db/schema'
import { eq, and } from 'drizzle-orm'
import { verifyQuizAnswer } from '../../../../../utils/quiz'
import { activeLearner } from '../../../../../utils/learner'
import { chapterContext, visitChapter, finishChapter, savedChapterProgress, hasPassedAll } from '../../../../../services/learning'
export default defineEventHandler(async event => {
  const body = (await readBody(event)) || {}
  const questionId = typeof body.questionId === 'string' ? body.questionId : ''
  const answer = typeof body.answer === 'string' ? body.answer : ''
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(questionId) || answer.length > 2000)
    throw createError({ statusCode: 400, message: '无效题目标识或答案过长' })
  const userId = activeLearner(event)
  return db.transaction(async tx => {
    const { chapter } = await chapterContext(tx, getRouterParam(event, 'slug') || '', getRouterParam(event, 'chapterSlug') || '', userId, true)
    const [question] = await tx.select({ answerHmac: chapterQuestions.answerHmac, normalization: chapterQuestions.normalization, errorHint: chapterQuestions.errorHint })
      .from(chapterQuestions).where(and(eq(chapterQuestions.chapterId, chapter.id), eq(chapterQuestions.id, questionId)))
    if (!question) throw createError({ statusCode: 404, message: '题目已更新，请刷新页面' })
    const correct = question.answerHmac ? verifyQuizAnswer(answer, question.answerHmac, question.normalization) : false
    if (userId) {
      await visitChapter(tx, userId, chapter.id)
      if (correct) {
        await tx.insert(questionCompletions).values({ userId, questionId }).onConflictDoNothing()
        if (await hasPassedAll(tx, userId, chapter.id)) await finishChapter(tx, userId, chapter.id)
      }
    }
    return { ok: true, data: { correct, errorHint: correct ? null : question.errorHint, progress: await savedChapterProgress(tx, chapter.id, userId) } }
  })
})
