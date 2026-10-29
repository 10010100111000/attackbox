/**
 * 靶场入口的展示信息。模块、章节与挑战都可能展示靶场,
 * 但只有模块/章节归属需要 scope,因此把展示字段独立出来。
 */
export type ConfiguredTarget = {
  status: 'configured'
  name: string
  description: string
  entryPort: number
  accessInstructions: string
}

export type TargetDisplay = null | { status: 'unavailable' } | ConfiguredTarget

export type LearningTarget =
  null | { status: 'unavailable' } | (ConfiguredTarget & { scope: 'module' | 'chapter' })
