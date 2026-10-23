export const normalizations = ['exact', 'trim', 'trim_casefold'] as const
export type Normalization = typeof normalizations[number]
export const difficulties = ['beginner', 'intermediate', 'advanced'] as const
export type Difficulty = typeof difficulties[number]
export interface ContentCategory { id: string; slug: string; name: string }
export interface QuestionInput {
  id?: string; questionKey?: string; position?: number; promptMarkdown: string
  answer?: string; clearAnswer?: boolean; errorHint?: string | null; normalization: Normalization
}
export interface ChapterInput {
  chapterId?: string; slug?: string; title: string; access: 'public' | 'authenticated'
  bodyMarkdown: string; questions?: QuestionInput[]
}
export interface ModuleInput {
  title?: string; slug?: string; summary?: string; descriptionMarkdown?: string
  difficulty?: typeof difficulties[number]; coverAssetId?: string | null; categoryId?: string | null
  lockVersion?: number; baseRevisionId?: string | null; chapters?: ChapterInput[]
}

// Canonical JSON response contracts, shared by the list and editor.
export interface AdminQuestion {
  id: string; questionKey: string; position: number; promptMarkdown: string
  errorHint: string | null; normalization: Normalization; hasAnswer: boolean
}
export interface AdminChapter {
  chapterId: string; slug: string; position: number; chapterRevisionId: string
  title: string; access: 'public' | 'authenticated'; bodyMarkdown: string
  isSealed: boolean; questions: AdminQuestion[]
}
export interface AdminRevision {
  id: string; moduleId: string; revisionNo: number; state: string; title: string | null
  summary: string | null; descriptionMarkdown?: string | null; difficulty: Difficulty
  categoryId: string | null; lockVersion: number; updatedAt: string; publishedAt: string | null
}
export interface AdminModuleListItem {
  id: string; slug: string; createdAt: string; archivedAt: string | null
  draft: (AdminRevision & { chapterCount: number }) | null; published: AdminRevision | null
  isPublished: boolean; hasDraft: boolean; chapterCount: number; publishedChapterCount: number
  category: ContentCategory | null
}
export interface AdminModuleDetail {
  module: { id: string; slug: string; publishedRevisionId: string | null; createdAt: string; archivedAt: string | null }
  draft: AdminRevision | null; published: AdminRevision | null; editable: AdminRevision | null
  isPublished: boolean; hasDraft: boolean; chapterCount: number; category: ContentCategory | null
  baseRevisionId: string | null; lockVersion: number; chapters: AdminChapter[]
}
