import { defineEventHandler, setResponseHeader } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { sandboxHub } from '../../../sandbox'
export default defineEventHandler(event => {
  setResponseHeader(event, 'Cache-Control', 'private, no-store')
  setResponseHeader(event, 'Vary', 'Cookie')
  requireAdmin(event)
  return { ok: true, data: sandboxHub().snapshot() }
})
