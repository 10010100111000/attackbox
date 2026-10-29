import { defineEventHandler, getRouterParam } from 'h3'
import { requireAdmin } from '../../../../utils/admin'
import { sandboxId, sandboxAction } from '../../../../utils/sandbox'
import { sandboxController } from '../../../../sandbox'
export default defineEventHandler(event =>
  sandboxAction(async () => {
    requireAdmin(event)
    await sandboxController().stopAdmin(sandboxId(getRouterParam(event, 'id')))
    return { ok: true }
  })
)
