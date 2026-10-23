import { createError, type H3Event } from 'h3'
export function activeLearner(event: H3Event) { return event.context.user?.status === 'active' ? event.context.user.id : undefined }
export function requireLearner(event: H3Event) {
  const id = activeLearner(event)
  if (!id) throw createError({ statusCode: 401, message: '请登录后保存学习进度' })
  return id
}
