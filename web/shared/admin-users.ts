import type { SandboxInstance } from './sandbox'
import type { LearningHistoryItem } from './learning'
export interface AdminUser {
  id: string
  email: string
  displayName: string
  role: string
  status: string
  createdAt: string
  lastLoginAt: string | null
  emailVerifiedAt: string | null
}
export interface AdminUserList {
  items: (AdminUser & { instance: SandboxInstance | null })[]
  total: number
  page: number
  pageSize: number
}
export interface AdminUserDetail {
  user: AdminUser
  activeSessions: number
  progress: { modules: number; visited: number; completed: number }
  history: LearningHistoryItem[]
  instance: SandboxInstance | null
}
export const userStatusLabels: Record<string, string> = {
  active: '正常',
  pending: '待验证',
  suspended: '已停用'
}
export const instanceStateLabels: Record<string, string> = {
  creating: '创建中',
  running: '运行中',
  stopping: '释放中',
  cleanup_failed: '清理待确认'
}
