export interface RepoModuleEntry {
  sourcePath: string
  folder: string
  title: string
  mdSha: string
  status: 'new' | 'modified' | 'up_to_date'
  changedReasons: string[]
  moduleId: string | null
  moduleSlug: string | null
  moduleStatus: 'published' | 'unpublished' | null
  syncedCommit: string | null
  syncedAt: string | null
}

export interface RepoScanReport {
  commit: string
  repository: string
  branch: string
  directory: string
  modules: RepoModuleEntry[]
}

export interface SyncModuleResult {
  synced: {
    moduleId: string
    title: string
    sourcePath: string
    action: 'created' | 'updated' | 'unchanged'
    downloadedImages: number
    reusedImages: number
  }[]
  commit: string
}
