import { defineEventHandler, readBody } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { saveModule } from '../../../services/content'
export default defineEventHandler(async event => {
  const admin = requireAdmin(event)
  const body = (await readBody(event)) || {}
  return { ok: true, data: await saveModule(null, admin.id, body) }
})
