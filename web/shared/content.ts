export const normalizations = ['exact', 'trim', 'trim_casefold'] as const
export type Normalization = typeof normalizations[number]
export const difficulties = ['beginner', 'intermediate', 'advanced'] as const
export type Difficulty = typeof difficulties[number]
export type ModuleStatus = 'unpublished' | 'published'
export interface ContentCategory { id: string; slug: string; name: string }
export interface QuestionInput {
  id?: string; promptMarkdown: string; answer?: string; clearAnswer?: boolean
  errorHint?: string | null; normalization: Normalization
}
export interface ChapterInput {
  chapterId?: string; slug?: string; title: string; access: 'public' | 'authenticated'
  bodyMarkdown: string; questions?: QuestionInput[]
}
export interface ModuleInput {
  title?: string; slug?: string; summary?: string; descriptionMarkdown?: string
  difficulty?: Difficulty; coverAssetId?: string | null; categoryId?: string | null
  lockVersion?: number; chapters?: ChapterInput[]
}
export interface AdminQuestion {
  id: string; position: number; promptMarkdown: string
  errorHint: string | null; normalization: Normalization; hasAnswer: boolean
}
export interface AdminChapter {
  chapterId: string; slug: string; position: number; title: string
  access: 'public' | 'authenticated'; bodyMarkdown: string; questions: AdminQuestion[]
}
export interface AdminModule {
  id: string; slug: string; title: string; summary: string; descriptionMarkdown: string
  difficulty: Difficulty; categoryId: string | null; coverAssetId: string | null
  status: ModuleStatus; lockVersion: number; createdAt: string; updatedAt: string; publishedAt: string | null
}
export type AdminModuleListItem = Omit<AdminModule, 'descriptionMarkdown'> & { chapterCount: number; category: ContentCategory | null }
export interface AdminModuleDetail {
  module: AdminModule; category: ContentCategory | null; chapters: AdminChapter[]
}
