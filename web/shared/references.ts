export interface ReferenceSummary {
  id: string
  slug: string
  title: string
  summary: string
  updatedAt: string
  publishedAt: string | null
}
export interface AdminReferenceSummary extends ReferenceSummary {
  status: 'unpublished' | 'published'
  lockVersion: number
  sourcePath: string
}
export interface AdminReference extends AdminReferenceSummary {
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

export interface ReferenceMetadata {
  id: string
  slug: string
  title: string
  summary: string
  revision: string
  outline: { id: string; text: string; level: number; chunk: number }[]
  chunks: { index: number; title: string; url: string }[]
}
export interface ReferenceReading {
  meta: ReferenceMetadata
  html: string
}

export interface ReferenceImportResult {
  id: string
  unchanged: boolean
  lockVersion: number
  status: 'unpublished' | 'published'
}
