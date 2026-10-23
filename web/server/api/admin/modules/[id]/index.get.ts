import { defineEventHandler, getRouterParam, createError } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { getModuleDetail } from '../../../../services/content'
export default defineEventHandler(async event => {
  requireAdmin(event)
  const id = getRouterParam(event, 'id')
  if (!id) throw createError({ statusCode: 400, message: '缺少 ID' })
  return { ok: true, data: await getModuleDetail(id) }
})
