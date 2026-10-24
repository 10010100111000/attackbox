import { defineEventHandler, getRouterParam } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { userId, userDetail } from '../../../../services/admin-users'
export default defineEventHandler(async event => {
  requireAdmin(event)
  return { ok: true, data: await userDetail(userId(getRouterParam(event, 'id'))) }
})
