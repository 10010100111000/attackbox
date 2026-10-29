import { type H3Event, createError } from 'h3'

export interface AdminUser {
  id: string
  email: string
  displayName: string
  role: 'user' | 'admin'
  status: 'pending' | 'active' | 'suspended'
}

/**
 * 校验当前请求上下文是否包含有效的管理员身份
 * 未登录返回 401，非管理员或非 active 返回 403
 */
export function requireAdmin(event: H3Event): AdminUser {
  const user = event.context.user

  if (!user) {
    throw createError({
      statusCode: 401,
      statusMessage: 'Unauthorized',
      message: '请先登录管理员账号'
    })
  }

  if (user.role !== 'admin' || user.status !== 'active') {
    throw createError({
      statusCode: 403,
      statusMessage: 'Forbidden',
      message: '需要系统管理员权限'
    })
  }

  return user
}
