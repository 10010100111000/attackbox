import { defineEventHandler, getQuery } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { listFeedback } from '../../../services/feedback'

export default defineEventHandler(async event => {
  requireAdmin(event)
  const query = getQuery(event)
  return { data: await listFeedback(query.page, query.status) }
})
