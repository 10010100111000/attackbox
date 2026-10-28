import { defineEventHandler, getRouterParam } from 'h3'
import { referenceDetail } from '../../../services/references'
export default defineEventHandler(async event => ({ data: await referenceDetail(getRouterParam(event, 'slug') || '') }))
