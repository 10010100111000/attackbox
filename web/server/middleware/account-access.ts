import {
  defineEventHandler,
  setResponseHeader,
  getHeader,
  getRequestURL,
  createError,
} from 'h3'
export default defineEventHandler((e) => {
  if (
    ![
      '/api/me/',
      '/api/admin/notifications',
      '/api/admin/runtime-policy',
      '/api/admin/learn-home',
      '/admin/learn-home',
      '/admin/runtime-policy',
      '/notifications',
      '/me',
      '/admin/notifications',
    ].some((p) => e.path.startsWith(p))
  )
    return
  setResponseHeader(e, 'Cache-Control', 'private, no-store')
  setResponseHeader(e, 'Vary', 'Cookie')
  if (['GET', 'HEAD'].includes(e.method)) return
  const origin = getHeader(e, 'origin')
  if (
    getHeader(e, 'sec-fetch-site') === 'cross-site' ||
    (origin && origin !== getRequestURL(e).origin)
  )
    throw createError({ statusCode: 403, message: '请从本站提交操作' })
})
