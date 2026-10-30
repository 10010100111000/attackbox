import { defineEventHandler, getQuery } from 'h3'
import { requireAdmin } from '../../utils/admin'
import { analyticsReport } from '../../services/site-analytics'
export default defineEventHandler(async event => {
  requireAdmin(event)
  return { data: await analyticsReport(Number(getQuery(event).days || 7)) }
})
