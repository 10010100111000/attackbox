import { defineEventHandler, readBody } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { inputAd, saveAdvertisement } from '../../../services/advertising'
export default defineEventHandler(async event => {
  requireAdmin(event)
  return { data: await saveAdvertisement(null, inputAd(await readBody(event))) }
})
