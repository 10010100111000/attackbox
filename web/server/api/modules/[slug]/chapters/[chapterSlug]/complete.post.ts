import { defineEventHandler, getRouterParam, createError } from 'h3'
import { db } from '../../../../../db'
import { requireLearner } from '../../../../../utils/learner'
import {
  chapterContext,
  visitChapter,
  finishChapter,
  savedChapterProgress,
  hasPassedAll
} from '../../../../../services/learning'
export default defineEventHandler(async event => {
  const userId = requireLearner(event)
  return db.transaction(async tx => {
    const { chapter } = await chapterContext(
      tx,
      getRouterParam(event, 'slug') || '',
      getRouterParam(event, 'chapterSlug') || '',
      userId,
      true
    )
    if (!chapter.completedAt && !(await hasPassedAll(tx, userId, chapter.id)))
      throw createError({ statusCode: 409, message: '请先通过本章全部练习' })
    await visitChapter(tx, userId, chapter.id)
    await finishChapter(tx, userId, chapter.id)
    return { ok: true, data: { progress: await savedChapterProgress(tx, chapter.id, userId) } }
  })
})
