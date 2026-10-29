import type { ComposeSettings } from './compose-labs'
export interface TargetTemplateInput {
  name: string
  description: string
  accessInstructions: string
  enabled: boolean
}
export interface TargetTemplate extends TargetTemplateInput {
  id: string
  lockVersion: number
  createdAt: string
  updatedAt: string
  bundleId: string
  revision: number
  settings: ComposeSettings
  variableKeys: string[]
}
export interface TargetTemplateList {
  items: TargetTemplate[]
  total: number
  page: number
  pageSize: number
}
