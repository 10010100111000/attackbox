import { defineEventHandler, getRouterParam, readBody } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { saveSandboxSettings } from '../../../../services/sandbox-settings'
export default defineEventHandler(async event => {
  const admin = requireAdmin(event)
  return {
    ok: true,
    data: await saveSandboxSettings(
      getRouterParam(event, 'id') || '',
      await readBody(event),
      admin.id
    )
  }
})
