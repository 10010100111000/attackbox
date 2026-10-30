import { defineEventHandler, getRouterParam } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { getAdvertisement } from '../../../services/advertising'
export default defineEventHandler(async event => {
  requireAdmin(event)
  return { data: await getAdvertisement(getRouterParam(event, 'id')!) }
})
