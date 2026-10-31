import { defineEventHandler, getRouterParam } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { userId, deleteUser } from '../../../../services/admin-users'

export default defineEventHandler(async event => {
  const actor = requireAdmin(event)
  await deleteUser(actor.id, userId(getRouterParam(event, 'id')))
  return { ok: true }
})
