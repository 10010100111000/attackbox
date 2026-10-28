import { defineEventHandler } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { sandboxHub } from '../../../sandbox'
export default defineEventHandler(event => {
  requireAdmin(event)
  return { ok: true, data: sandboxHub().snapshot() }
})
