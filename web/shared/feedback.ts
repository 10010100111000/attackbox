export const feedbackCategories = [
  { value: 'bug', label: '遇到问题' },
  { value: 'content', label: '内容纠错' },
  { value: 'suggestion', label: '功能建议' }
] as const

export const feedbackStatuses = [
  { value: 'new', label: '未处理' },
  { value: 'in_progress', label: '处理中' },
  { value: 'resolved', label: '已解决' }
] as const

export type FeedbackCategory = (typeof feedbackCategories)[number]['value']
export type FeedbackStatus = (typeof feedbackStatuses)[number]['value']

export interface FeedbackItem {
  id: string
  category: FeedbackCategory
  body: string
  sourcePath: string | null
  status: FeedbackStatus
  createdAt: string
  updatedAt: string
  author: { id: string | null; name: string; email: string | null }
}

export interface FeedbackList {
  items: FeedbackItem[]
  total: number
  page: number
  pageSize: number
  counts: Record<FeedbackStatus, number>
}
