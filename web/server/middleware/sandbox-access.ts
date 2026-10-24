import { defineEventHandler, getRequestURL, setResponseHeader, getHeader, createError } from 'h3'
export default defineEventHandler(event => {
  const path = event.path.split('?')[0] || ''
  if (!(path.startsWith('/api/targets') || path.startsWith('/api/challenges') || path.startsWith('/api/admin/challenges') || path.startsWith('/api/me/challenges') || path.startsWith('/admin/challenges') || path.startsWith('/challenges') || path.startsWith('/api/sandbox/') || path.startsWith('/api/admin/target-templates') || path.startsWith('/admin/target-templates') || path.startsWith('/api/admin/instances') || path.startsWith('/api/admin/nodes') || path.startsWith('/api/admin/routes') || path.startsWith('/api/routes') || path.startsWith('/learn/routes') || path.startsWith('/admin/routes') || path.startsWith('/api/admin/users') || path.startsWith('/admin/users') || path.startsWith('/admin/instances'))) return
  setResponseHeader(event, 'Cache-Control', 'private, no-store')
  setResponseHeader(event, 'Vary', 'Cookie')
  if (event.method === 'GET' || event.method === 'HEAD') return
  const origin = getHeader(event, 'origin')
  if (getHeader(event, 'sec-fetch-site') === 'cross-site' || (origin && origin !== getRequestURL(event).origin)) throw createError({ statusCode: 403, message: '请从本站提交操作' })
})
