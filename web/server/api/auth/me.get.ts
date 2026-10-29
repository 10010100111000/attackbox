import { defineEventHandler, createError } from 'h3'

export default defineEventHandler(event => {
  const user = event.context.user

  if (!user) {
    throw createError({
      statusCode: 401,
      statusMessage: '未登录或会话已失效'
    })
  }

  return { user }
})
