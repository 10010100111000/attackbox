import { defineEventHandler, getRouterParam } from 'h3'
import { selectAdvertisement } from '../../services/advertising'
export default defineEventHandler(async event => ({
  data: await selectAdvertisement(getRouterParam(event, 'placement'))
}))
