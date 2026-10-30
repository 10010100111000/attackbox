import { defineEventHandler, getRouterParam } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { adminReferenceDetail } from '../../../../services/references'
export default defineEventHandler(async event => {
  requireAdmin(event)
  return { data: await adminReferenceDetail(getRouterParam(event, 'id') || '') }
})
