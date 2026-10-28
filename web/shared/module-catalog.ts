import type { ContentCategory, Difficulty } from './content'

export interface PublicModule {
  id: string
  slug: string
  title: string
  summary: string
  difficulty: Difficulty
  categoryId: string | null
  category: ContentCategory | null
  coverAssetId: string | null
  publishedAt: string | null
  updatedAt: string
  durationMinutes: number
  chapterCount: number
}
