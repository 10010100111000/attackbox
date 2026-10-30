import { defineEventHandler, getRouterParam } from 'h3'
import { requireAdmin } from '../../../../../utils/admin'
import { adminReferencePreview } from '../../../../../services/references'
export default defineEventHandler(async event => {
  requireAdmin(event)
  return { data: await adminReferencePreview(getRouterParam(event, 'id') || '') }
})
