import { defineEventHandler, getQuery } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { listTargetTemplates } from '../../../services/target-templates'
export default defineEventHandler(async e => {
  requireAdmin(e)
  return { ok: true, data: await listTargetTemplates(getQuery(e)) }
})
