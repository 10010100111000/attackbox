export interface ReferenceSummary {
  id: string
  slug: string
  title: string
  summary: string
  updatedAt: string
  publishedAt: string | null
}
export interface ReferenceDetail extends ReferenceSummary {
  bodyMarkdown: string
  imageUrls: Record<string, string>
}
export interface AdminReferenceSummary extends ReferenceSummary {
  status: 'unpublished' | 'published'
  lockVersion: number
  sourcePath: string
}
export interface AdminReference extends ReferenceDetail, AdminReferenceSummary {
  status: 'unpublished' | 'published'
  lockVersion: number
  sourcePath: string
  sourceCommit: string
}
export interface ReferencePage<T = ReferenceSummary> {
  items: T[]
  total: number
  page: number
  pageSize: number
}
export interface ReferenceScan {
  commit: string
  version: number
  items: {
    sourcePath: string
    title: string
    id: string | null
    lockVersion: number
    state: 'new' | 'changed' | 'unchanged' | 'missing'
  }[]
}
