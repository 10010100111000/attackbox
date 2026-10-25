import { defineEventHandler, getRouterParam, createError } from 'h3'
import { eq, and, asc } from 'drizzle-orm'
import {contentImageUrls} from '../../../services/content-images'
import { learningOutline } from '../../../services/learning'
import { activeLearner } from '../../../utils/learner'
import { summarizeProgress } from '../../../../shared/learning'
import { db } from '../../../db'
import { modules, contentCategories, modulePrerequisites } from '../../../db/schema'
export default defineEventHandler(async event => db.transaction(async tx => {
  const slug = getRouterParam(event, 'slug') || ''
  const [mod] = await tx.select({ id: modules.id, slug: modules.slug, title: modules.title,
    summary: modules.summary, descriptionMarkdown: modules.descriptionMarkdown, learningOutcomes: modules.learningOutcomes,
    category: { id: contentCategories.id, slug: contentCategories.slug, name: contentCategories.name },
    difficulty: modules.difficulty, categoryId: modules.categoryId,
    coverAssetId: modules.coverAssetId, publishedAt: modules.publishedAt,
  }).from(modules).leftJoin(contentCategories, eq(modules.categoryId, contentCategories.id)).where(and(eq(modules.slug, slug), eq(modules.status, 'published')))
  if (!mod) throw createError({ statusCode: 404, message: '模块不存在或尚未发布' })
  const userId = activeLearner(event)
  const list = await learningOutline(tx, mod.id, userId)
  const prerequisites = await tx.select({ id: modules.id, slug: modules.slug, title: modules.title }).from(modulePrerequisites).innerJoin(modules, eq(modulePrerequisites.prerequisiteId, modules.id)).where(and(eq(modulePrerequisites.moduleId, mod.id), eq(modules.status, 'published'))).orderBy(asc(modulePrerequisites.position))
  return { ok: true, data: { module: mod, imageUrls:await contentImageUrls(tx,mod.id), prerequisites, chapters: list, progress: userId ? summarizeProgress(list) : null } }
}, { isolationLevel: 'repeatable read', accessMode: 'read only' }))
