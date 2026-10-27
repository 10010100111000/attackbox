export interface NotificationItem {
  id: string
  kind: 'announcement' | 'message'
  sender: { id: string | null; name: string }
  title: string
  body: string
  link: string | null
  publishedAt: string
  withdrawnAt?: string | null
  readAt?: string | null
}
export interface NotificationList {
  items: NotificationItem[]
  total: number
  page: number
  pageSize: number
  unread: number
}
