import { defineEventHandler, getRequestURL, setResponseHeader, getHeader, createError } from 'h3'
export default defineEventHandler(event => {
  const path = event.path.split('?')[0] || ''
  if (!(path.startsWith('/api/sandbox/') || path.startsWith('/api/admin/instances') || path.startsWith('/api/admin/nodes') || path.startsWith('/admin/instances'))) return
  setResponseHeader(event, 'Cache-Control', 'private, no-store')
  setResponseHeader(event, 'Vary', 'Cookie')
  if (event.method === 'GET' || event.method === 'HEAD') return
  const origin = getHeader(event, 'origin')
  if (getHeader(event, 'sec-fetch-site') === 'cross-site' || (origin && origin !== getRequestURL(event).origin)) throw createError({ statusCode: 403, message: '请从本站提交操作' })
})
