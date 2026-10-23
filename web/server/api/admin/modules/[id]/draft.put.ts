import { defineEventHandler, readBody, getRouterParam, createError } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { saveModule } from '../../../../services/content'
export default defineEventHandler(async event => {
  const admin = requireAdmin(event)
  const id = getRouterParam(event, 'id')
  if (!id) throw createError({ statusCode: 400, message: '缺少 ID' })
  const body = (await readBody(event)) || {}
  const result = await saveModule(id, admin.id, body)
  return { ok: true, data: result.draft }
})
