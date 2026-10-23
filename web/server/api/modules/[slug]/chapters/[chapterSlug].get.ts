import { defineEventHandler, getRouterParam, createError } from 'h3'
import { eq, and, asc } from 'drizzle-orm'
import { db } from '../../../../db'
import { modules, chapters, chapterQuestions } from '../../../../db/schema'
export default defineEventHandler(async event => db.transaction(async tx => {
  const [chapter] = await tx.select({ id: chapters.id, slug: chapters.slug, title: chapters.title,
    access: chapters.access, position: chapters.position, bodyMarkdown: chapters.bodyMarkdown,
  }).from(chapters).innerJoin(modules, eq(chapters.moduleId, modules.id)).where(and(
    eq(modules.slug, getRouterParam(event, 'slug') || ''), eq(modules.status, 'published'),
    eq(chapters.slug, getRouterParam(event, 'chapterSlug') || ''),
  ))
  if (!chapter) throw createError({ statusCode: 404, message: '章节不存在或模块尚未发布' })
  if (chapter.access === 'authenticated' && event.context.user?.status !== 'active')
    throw createError({ statusCode: 401, message: '请登录后阅读本章节' })
  const questions = await tx.select({ id: chapterQuestions.id, position: chapterQuestions.position,
    promptMarkdown: chapterQuestions.promptMarkdown, errorHint: chapterQuestions.errorHint,
    normalization: chapterQuestions.normalization,
  }).from(chapterQuestions).where(eq(chapterQuestions.chapterId, chapter.id)).orderBy(asc(chapterQuestions.position))
  return { ok: true, data: { chapter, questions } }
}, { isolationLevel: 'repeatable read', accessMode: 'read only' }))
