import { defineEventHandler, getRouterParam, readBody } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { deleteAdvertisement } from '../../../services/advertising'
export default defineEventHandler(async event => {
  requireAdmin(event)
  await deleteAdvertisement(getRouterParam(event, 'id')!, (await readBody(event))?.lockVersion)
  return { deleted: true }
})
