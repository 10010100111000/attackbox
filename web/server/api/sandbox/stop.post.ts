import { defineEventHandler, readBody } from 'h3'
import { sandboxController } from '../../sandbox'
import { sandboxAction, requireSandboxUser, sandboxId } from '../../utils/sandbox'
export default defineEventHandler(event => sandboxAction(async () => {
  const userId = requireSandboxUser(event), body = await readBody(event)
  await sandboxController().stopOwned(userId, sandboxId(body?.sandboxId))
  return { ok: true, data: sandboxController().status(userId) }
}))
