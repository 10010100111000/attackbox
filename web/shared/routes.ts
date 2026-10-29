export interface RouteModule {
  id: string
  title: string
  slug: string
  summary: string
  difficulty: string
  status: string
  chapterCount: number
  durationMinutes: number
  completedChapters: number
}
export interface RouteSummary {
  id: string
  title: string
  summary: string
  status: string
  lockVersion: number
  moduleCount: number
}
export interface LearningRoute extends RouteSummary {
  chapterCount: number
  durationMinutes: number
  completedChapters: number
  modules: RouteModule[]
}
