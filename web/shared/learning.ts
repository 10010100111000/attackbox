export type LockReason = 'login' | 'prerequisite' | null
export interface LearningChapter {
  id: string
  slug: string
  title: string
  access: 'public' | 'authenticated'
  position: number
  durationMinutes: number | null
  questionsCount: number
  completedAt: string | null
  lastVisitedAt: string | null
  lockReason: LockReason
  isLocked: boolean
}
export interface ModuleProgress {
  completedChapters: number
  totalChapters: number
  percent: number
  isCompleted: boolean
  resumeChapterSlug: string | null
}
export interface ChapterProgress {
  completedAt: string | null
  passedQuestionIds: string[]
  isSaved: boolean
}
export interface LearningHistoryItem {
  moduleId: string
  slug: string
  title: string
  chapterTitle: string
  lastVisitedAt: string
  progress: ModuleProgress
}
export function applyChapterAccess<T extends { access: string; completedAt: string | null }>(
  list: T[],
  authenticated: boolean
): (T & { lockReason: LockReason; isLocked: boolean })[] {
  let precedingComplete = true
  return list.map(chapter => {
    const lockReason: LockReason =
      chapter.access === 'public'
        ? null
        : !authenticated
          ? 'login'
          : chapter.completedAt || precedingComplete
            ? null
            : 'prerequisite'
    precedingComplete = precedingComplete && !!chapter.completedAt
    return { ...chapter, lockReason, isLocked: lockReason !== null }
  })
}
export function summarizeProgress(
  list: Pick<LearningChapter, 'slug' | 'completedAt' | 'lastVisitedAt' | 'isLocked'>[]
): ModuleProgress {
  const completedChapters = list.filter(c => c.completedAt).length
  const isCompleted = list.length > 0 && completedChapters === list.length
  const unfinished = list.filter(c => !c.completedAt && !c.isLocked)
  const last = [...unfinished]
    .filter(c => c.lastVisitedAt)
    .sort((a, b) => b.lastVisitedAt!.localeCompare(a.lastVisitedAt!))[0]
  return {
    completedChapters,
    totalChapters: list.length,
    percent: list.length ? Math.round((completedChapters / list.length) * 100) : 0,
    isCompleted,
    resumeChapterSlug: (last || unfinished[0] || list.find(c => !c.isLocked))?.slug || null
  }
}
