import { defineEventHandler, getRouterParam } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { userId, revokeUserSessions } from '../../../../services/admin-users'
export default defineEventHandler(async event => {
  const actor = requireAdmin(event)
  await revokeUserSessions(actor.id, userId(getRouterParam(event, 'id')))
  return { ok: true }
})
