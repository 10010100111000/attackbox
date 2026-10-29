export interface ChallengeStage {
  id: string
  position: number
  title: string
  promptMarkdown: string
  hint: string
  passedAt: string | null
  locked: boolean
  flag?: string
}
export interface ChallengeSummary {
  id: string
  title: string
  summary: string
  category: string
  difficulty: string
  status: string
  lockVersion: number
  totalStages: number
  completedStages: number
  lastPassedAt?: string | null
}
export interface Challenge extends ChallengeSummary {
  bodyMarkdown: string
  targetTemplateId?: string | null
  stages: ChallengeStage[]
  isCompleted: boolean
  target:
    | null
    | { status: 'unavailable' }
    | {
        status: 'configured'
        name: string
        description: string
        entryPort: number
        accessInstructions: string
      }
}
