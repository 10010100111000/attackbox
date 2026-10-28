import { defineEventHandler, setResponseHeader, appendResponseHeader } from 'h3'
export default defineEventHandler(event => {
  const path = event.path.split('?')[0] || ''
  if (path === '/references' || path.startsWith('/references/') || path === '/api/references' || path.startsWith('/api/references/') || path === '/admin/nodes' || path.startsWith('/admin/nodes/') || path === '/learn' || path.startsWith('/learn/') || path === '/api/modules' || path.startsWith('/api/modules/') || path.startsWith('/api/me/learning')) {
    setResponseHeader(event, 'Cache-Control', 'private, no-store')
    appendResponseHeader(event, 'Vary', 'Cookie')
  }
})
