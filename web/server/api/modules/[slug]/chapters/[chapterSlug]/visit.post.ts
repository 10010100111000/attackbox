import { defineEventHandler, getRouterParam } from 'h3'
import { db } from '../../../../../db'
import { requireLearner } from '../../../../../utils/learner'
import { chapterContext, visitChapter } from '../../../../../services/learning'
export default defineEventHandler(async event => {
  const userId = requireLearner(event)
  return db.transaction(async tx => {
    const { chapter } = await chapterContext(tx, getRouterParam(event, 'slug') || '', getRouterParam(event, 'chapterSlug') || '', userId, true)
    await visitChapter(tx, userId, chapter.id)
    return { ok: true }
  })
})
