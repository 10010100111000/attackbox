import { defineEventHandler, getRouterParam } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { withdrawNotification } from '../../../../services/notifications'
export default defineEventHandler((e) => {
  requireAdmin(e)
  return withdrawNotification(getRouterParam(e, 'id'))
})
