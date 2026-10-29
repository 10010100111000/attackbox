import { defineEventHandler, readBody } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { publishNotification } from '../../../services/notifications'
export default defineEventHandler(async e => ({
  data: await publishNotification(requireAdmin(e).id, await readBody(e))
}))
