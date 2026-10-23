import { defineEventHandler, getRouterParam, createError } from 'h3'
import { eq, and, asc, sql } from 'drizzle-orm'
import { db } from '../../../db'
import { modules, chapters, chapterQuestions } from '../../../db/schema'
export default defineEventHandler(async event => db.transaction(async tx => {
  const slug = getRouterParam(event, 'slug') || ''
  const [mod] = await tx.select({ id: modules.id, slug: modules.slug, title: modules.title,
    summary: modules.summary, descriptionMarkdown: modules.descriptionMarkdown,
    difficulty: modules.difficulty, categoryId: modules.categoryId,
    coverAssetId: modules.coverAssetId, publishedAt: modules.publishedAt,
  }).from(modules).where(and(eq(modules.slug, slug), eq(modules.status, 'published')))
  if (!mod) throw createError({ statusCode: 404, message: '模块不存在或尚未发布' })
  const list = await tx.select({ id: chapters.id, slug: chapters.slug, title: chapters.title,
    access: chapters.access, position: chapters.position,
    questionsCount: sql<number>`(select count(*)::int from ${chapterQuestions} where ${chapterQuestions.chapterId} = ${chapters.id})`,
  }).from(chapters).where(eq(chapters.moduleId, mod.id)).orderBy(asc(chapters.position))
  return { ok: true, data: { module: mod, chapters: list.map(ch => ({ ...ch,
    isLocked: ch.access === 'authenticated' && event.context.user?.status !== 'active',
  })) } }
}, { isolationLevel: 'repeatable read', accessMode: 'read only' }))
