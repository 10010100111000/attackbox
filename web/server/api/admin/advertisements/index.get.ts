import { defineEventHandler, getQuery } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { listAdvertisements } from '../../../services/advertising'
export default defineEventHandler(async event => {
  requireAdmin(event)
  const query = getQuery(event)
  return {
    data: await listAdvertisements(Number(query.page || 1), String(query.search || '').trim())
  }
})
