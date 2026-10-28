import { defineEventHandler, getRouterParam } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { referenceDetail } from '../../../../services/references'
export default defineEventHandler(async event => { requireAdmin(event); return { data: await referenceDetail(getRouterParam(event, 'id') || '', true) } })
