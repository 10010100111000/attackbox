import { defineEventHandler, getQuery } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { listReferences } from '../../../services/references'
export default defineEventHandler(async event => {
  requireAdmin(event)
  return { data: await listReferences(getQuery(event).page, true) }
})
