import { defineEventHandler, getRouterParam, readBody } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { changeReference } from '../../../../services/references'
export default defineEventHandler(async event => {
  requireAdmin(event)
  return {
    data: await changeReference(getRouterParam(event, 'id') || '', (await readBody(event)) || {})
  }
})
