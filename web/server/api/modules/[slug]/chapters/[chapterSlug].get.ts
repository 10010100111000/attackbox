import { defineEventHandler, getRouterParam } from 'h3'
import { eq, asc } from 'drizzle-orm'
import { db } from '../../../../db'
import { chapters, chapterQuestions } from '../../../../db/schema'
import { activeLearner } from '../../../../utils/learner'
import { chapterContext, savedChapterProgress } from '../../../../services/learning'
export default defineEventHandler(async event => db.transaction(async tx => {
  const userId = activeLearner(event)
  const { chapter: accessible } = await chapterContext(tx, getRouterParam(event, 'slug') || '', getRouterParam(event, 'chapterSlug') || '', userId)
  const [chapter] = await tx.select({ id: chapters.id, slug: chapters.slug, title: chapters.title,
    access: chapters.access, position: chapters.position, durationMinutes: chapters.durationMinutes, bodyMarkdown: chapters.bodyMarkdown,
  }).from(chapters).where(eq(chapters.id, accessible.id))
  const questions = await tx.select({ id: chapterQuestions.id, position: chapterQuestions.position,
    promptMarkdown: chapterQuestions.promptMarkdown, errorHint: chapterQuestions.errorHint, normalization: chapterQuestions.normalization,
  }).from(chapterQuestions).where(eq(chapterQuestions.chapterId, chapter.id)).orderBy(asc(chapterQuestions.position))
  return { ok: true, data: { chapter, questions, progress: await savedChapterProgress(tx, chapter.id, userId) } }
}, { isolationLevel: 'repeatable read', accessMode: 'read only' }))
