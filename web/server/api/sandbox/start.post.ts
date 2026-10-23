import { defineEventHandler, createError } from 'h3'
import { sandboxController } from '../../sandbox'
import { sandboxAction, requireSandboxUser, sandboxIdentity } from '../../utils/sandbox'
export default defineEventHandler(event => sandboxAction(async () => {
  const userId = requireSandboxUser(event), controller = sandboxController()
  if (!controller.status(userId).available) throw createError({ statusCode: 503, message: '练习环境尚未开放' })
  return { ok: true, data: await controller.start(userId, await sandboxIdentity(userId)) }
}))
