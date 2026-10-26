export type ChapterTargetMode = 'inherit' | 'none' | 'custom'
export const normalizations = ['exact', 'trim', 'trim_casefold'] as const
export type Normalization = typeof normalizations[number]
export const difficulties = ['beginner', 'intermediate', 'advanced'] as const
export type Difficulty = typeof difficulties[number]
export type ModuleStatus = 'unpublished' | 'published'
export interface ContentCategory { id: string; slug: string; name: string }
export interface QuestionInput {
  id?: string; promptMarkdown: string; answer?: string | null
  errorHint?: string | null; normalization: Normalization
}
export interface ChapterInput {
  chapterId?: string; slug?: string; title: string; access: 'public' | 'authenticated'
  bodyMarkdown: string; targetMode?: ChapterTargetMode; targetTemplateId?: string | null; durationMinutes?: number | null; questions?: QuestionInput[]
}
export interface ModuleInput {
  title?: string; slug?: string; summary?: string; descriptionMarkdown?: string
  difficulty?: Difficulty; categoryId?: string | null
  lockVersion?: number; chapters?: ChapterInput[]; learningOutcomes?: string[]
  defaultTargetTemplateId?: string | null
  prerequisiteModuleIds?: string[]
}
export interface AdminQuestion {
  id: string; position: number; promptMarkdown: string
  answer: string | null; errorHint: string | null; normalization: Normalization; hasAnswer: boolean
}
export interface AdminChapter {
  chapterId: string; slug: string; position: number; title: string
  access: 'public' | 'authenticated'; bodyMarkdown: string; targetMode: ChapterTargetMode; targetTemplateId: string | null; durationMinutes: number | null; questions: AdminQuestion[]
}
export interface AdminModule {
  id: string; slug: string; title: string; summary: string; descriptionMarkdown: string
  learningOutcomes: string[]
  defaultTargetTemplateId: string | null
  difficulty: Difficulty; categoryId: string | null; coverAssetId: string | null
  status: ModuleStatus; lockVersion: number; createdAt: string; updatedAt: string; publishedAt: string | null
}
export type AdminModuleListItem = Omit<AdminModule, 'descriptionMarkdown' | 'learningOutcomes'> & { source?:{path:string;commit:string|null}|null; chapterCount: number; category: ContentCategory | null }
export interface AdminModuleDetail {
  imageUrls?:Record<string,string>
  source?: {path:string;commit:string|null;url?:string|null;syncedAt:string}|null
  module: AdminModule; category: ContentCategory | null; chapters: AdminChapter[]; prerequisites: PrerequisiteModule[]
}

export interface PrerequisiteModule { id: string; slug: string; title: string; status: ModuleStatus }
