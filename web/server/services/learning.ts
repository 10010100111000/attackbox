import { and, asc, eq, sql, inArray } from 'drizzle-orm'
import { createError } from 'h3'
import { db } from '../db'
import { modules, chapters, chapterQuestions, chapterProgress, questionCompletions } from '../db/schema'
import { applyChapterAccess, summarizeProgress, type ChapterProgress, type LearningChapter } from '../../shared/learning'
export type LearningTx = typeof db | Parameters<Parameters<typeof db.transaction>[0]>[0]
export async function learningOutline(tx: LearningTx, moduleId: string, userId?: string): Promise<LearningChapter[]> {
  const rows = await tx.select({ id: chapters.id, slug: chapters.slug, title: chapters.title,
    access: chapters.access, position: chapters.position, durationMinutes: chapters.durationMinutes,
    questionsCount: sql<number>`(select count(*)::int from ${chapterQuestions} where ${chapterQuestions.chapterId} = ${chapters.id})`,
    completedAt: chapterProgress.completedAt, lastVisitedAt: chapterProgress.lastVisitedAt,
  }).from(chapters).leftJoin(chapterProgress, and(eq(chapterProgress.chapterId, chapters.id), userId ? eq(chapterProgress.userId, userId) : sql`false`))
    .where(eq(chapters.moduleId, moduleId)).orderBy(asc(chapters.position))
  return applyChapterAccess(rows.map(r => ({ ...r, completedAt: r.completedAt?.toISOString() || null, lastVisitedAt: r.lastVisitedAt?.toISOString() || null })), !!userId)
}
export async function chapterContext(tx: LearningTx, moduleSlug: string, chapterSlug: string, userId?: string, writing = false) {
  const query = tx.select({ id: modules.id }).from(modules).where(and(eq(modules.slug, moduleSlug), eq(modules.status, 'published')))
  // Content writes lock this same module row before modifying chapters/questions.
  const [mod] = await (writing ? query.for('share') : query)
  if (!mod) throw createError({ statusCode: 404, message: '模块不存在或尚未发布' })
  if (writing && userId) await tx.execute(sql`select pg_advisory_xact_lock(167923, hashtext(${userId + ':' + mod.id}))`)
  const outline = await learningOutline(tx, mod.id, userId)
  const chapter = outline.find(c => c.slug === chapterSlug)
  if (!chapter) throw createError({ statusCode: 404, message: '章节不存在或尚未发布' })
  if (chapter.lockReason === 'login') throw createError({ statusCode: 401, message: '请登录后阅读本章节' })
  if (chapter.lockReason === 'prerequisite') throw createError({ statusCode: 403, message: '请先完成前面的章节后继续学习' })
  return { chapter, outline }
}
export async function savedChapterProgress(tx: LearningTx, chapterId: string, userId?: string): Promise<ChapterProgress> {
  if (!userId) return { completedAt: null, passedQuestionIds: [], isSaved: false }
  const [progress] = await tx.select({ completedAt: chapterProgress.completedAt }).from(chapterProgress).where(and(eq(chapterProgress.chapterId, chapterId), eq(chapterProgress.userId, userId)))
  const passed = await tx.select({ id: questionCompletions.questionId }).from(questionCompletions)
    .innerJoin(chapterQuestions, eq(chapterQuestions.id, questionCompletions.questionId))
    .where(and(eq(questionCompletions.userId, userId), eq(chapterQuestions.chapterId, chapterId)))
  return { completedAt: progress?.completedAt?.toISOString() || null, passedQuestionIds: passed.map(p => p.id), isSaved: true }
}
export async function visitChapter(tx: LearningTx, userId: string, chapterId: string) {
  await tx.insert(chapterProgress).values({ userId, chapterId }).onConflictDoUpdate({ target: [chapterProgress.userId, chapterProgress.chapterId], set: { lastVisitedAt: new Date() } })
}
export async function finishChapter(tx: LearningTx, userId: string, chapterId: string) {
  await tx.insert(chapterProgress).values({ userId, chapterId, completedAt: new Date() }).onConflictDoUpdate({
    target: [chapterProgress.userId, chapterProgress.chapterId],
    set: { completedAt: sql`coalesce(${chapterProgress.completedAt}, now())` },
  })
}
export async function hasPassedAll(tx: LearningTx, userId: string, chapterId: string) {
  const [counts] = await tx.select({ missing: sql<number>`count(*)::int` }).from(chapterQuestions)
    .leftJoin(questionCompletions, and(eq(questionCompletions.questionId, chapterQuestions.id), eq(questionCompletions.userId, userId)))
    .where(and(eq(chapterQuestions.chapterId, chapterId), sql`${questionCompletions.questionId} is null`))
  return counts.missing === 0
}
export async function learningHistory(userId: string) {
  return db.transaction(async tx => {
    // Two batched queries; progress derives from current published chapters, never stale module counters.
    const touched = await tx.select({ moduleId: modules.id, slug: modules.slug, title: modules.title,
      lastVisitedAt: sql<Date>`max(${chapterProgress.lastVisitedAt})`,
    }).from(chapterProgress).innerJoin(chapters, eq(chapters.id, chapterProgress.chapterId)).innerJoin(modules, eq(modules.id, chapters.moduleId))
      .where(and(eq(chapterProgress.userId, userId), eq(modules.status, 'published'))).groupBy(modules.id)
      .orderBy(sql`max(${chapterProgress.lastVisitedAt}) desc`).limit(20)
    if (!touched.length) return []
    const all = await tx.select({ moduleId: chapters.moduleId, slug: chapters.slug, title: chapters.title, access: chapters.access, position: chapters.position,
      completedAt: chapterProgress.completedAt, lastVisitedAt: chapterProgress.lastVisitedAt,
    }).from(chapters).leftJoin(chapterProgress, and(eq(chapterProgress.chapterId, chapters.id), eq(chapterProgress.userId, userId)))
      .where(inArray(chapters.moduleId, touched.map(m => m.moduleId))).orderBy(asc(chapters.position))
    return touched.map(mod => {
      const list = applyChapterAccess(all.filter(c => c.moduleId === mod.moduleId).map(c => ({ ...c, completedAt: c.completedAt?.toISOString() || null, lastVisitedAt: c.lastVisitedAt?.toISOString() || null })), true)
      const progress = summarizeProgress(list)
      return { ...mod, lastVisitedAt: new Date(mod.lastVisitedAt).toISOString(), progress, chapterTitle: list.find(c => c.slug === progress.resumeChapterSlug)?.title || '' }
    })
  }, { isolationLevel: 'repeatable read', accessMode: 'read only' })
}
