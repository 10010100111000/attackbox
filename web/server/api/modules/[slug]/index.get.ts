import { defineEventHandler, getRouterParam, createError } from 'h3'
import { eq, and, asc, sql } from 'drizzle-orm'
import { db } from '../../../db'
import { modules, chapters, chapterQuestions, contentCategories, modulePrerequisites } from '../../../db/schema'
export default defineEventHandler(async event => db.transaction(async tx => {
  const slug = getRouterParam(event, 'slug') || ''
  const [mod] = await tx.select({ id: modules.id, slug: modules.slug, title: modules.title,
    summary: modules.summary, descriptionMarkdown: modules.descriptionMarkdown, learningOutcomes: modules.learningOutcomes,
    category: { id: contentCategories.id, slug: contentCategories.slug, name: contentCategories.name },
    difficulty: modules.difficulty, categoryId: modules.categoryId,
    coverAssetId: modules.coverAssetId, publishedAt: modules.publishedAt,
  }).from(modules).leftJoin(contentCategories, eq(modules.categoryId, contentCategories.id)).where(and(eq(modules.slug, slug), eq(modules.status, 'published')))
  if (!mod) throw createError({ statusCode: 404, message: '模块不存在或尚未发布' })
  const list = await tx.select({ id: chapters.id, slug: chapters.slug, title: chapters.title,
    access: chapters.access, position: chapters.position, durationMinutes: chapters.durationMinutes,
    questionsCount: sql<number>`(select count(*)::int from ${chapterQuestions} where ${chapterQuestions.chapterId} = ${chapters.id})`,
  }).from(chapters).where(eq(chapters.moduleId, mod.id)).orderBy(asc(chapters.position))
  const prerequisites = await tx.select({ id: modules.id, slug: modules.slug, title: modules.title }).from(modulePrerequisites).innerJoin(modules, eq(modulePrerequisites.prerequisiteId, modules.id)).where(and(eq(modulePrerequisites.moduleId, mod.id), eq(modules.status, 'published'))).orderBy(asc(modulePrerequisites.position))
  return { ok: true, data: { module: mod, prerequisites, chapters: list.map(ch => ({ ...ch,
    isLocked: ch.access === 'authenticated' && event.context.user?.status !== 'active',
  })) } }
}, { isolationLevel: 'repeatable read', accessMode: 'read only' }))
