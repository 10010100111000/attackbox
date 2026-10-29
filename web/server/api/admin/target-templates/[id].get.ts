import { defineEventHandler, getRouterParam } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { getTargetTemplate } from '../../../services/target-templates'
export default defineEventHandler(async e => {
  requireAdmin(e)
  return { ok: true, data: await getTargetTemplate(getRouterParam(e, 'id')!) }
})
