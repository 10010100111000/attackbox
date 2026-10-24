import { defineEventHandler, getRouterParam, readBody, createError } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { userId, changeUserStatus } from '../../../../services/admin-users'
export default defineEventHandler(async event => {
  const actor = requireAdmin(event), id = userId(getRouterParam(event, 'id')), body = await readBody(event)
  if (typeof body?.suspended !== 'boolean') throw createError({ statusCode: 400, message: '请选择停用或恢复账号' })
  return { ok: true, data: await changeUserStatus(actor.id, id, body.suspended) }
})
