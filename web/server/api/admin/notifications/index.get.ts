import { defineEventHandler, getQuery } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { listNotifications } from '../../../services/notifications'
export default defineEventHandler(async e => ({
  data: await listNotifications(requireAdmin(e).id, getQuery(e).page, true)
}))
