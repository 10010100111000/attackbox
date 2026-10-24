import { defineEventHandler, getQuery } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { listUsers } from '../../../services/admin-users'
export default defineEventHandler(async event => {
  requireAdmin(event)
  return { ok: true, data: await listUsers(getQuery(event)) }
})
