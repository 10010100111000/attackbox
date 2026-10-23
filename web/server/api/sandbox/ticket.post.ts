import { defineEventHandler, readBody, createError } from 'h3'
import { sandboxController } from '../../sandbox'
import { sandboxAction, requireSandboxUser, sandboxId } from '../../utils/sandbox'
export default defineEventHandler(event => sandboxAction(async () => {
  const userId = requireSandboxUser(event), body = await readBody(event)
  const stream = body?.stream ?? 'cli'
  if (stream !== 'cli' && stream !== 'gui') throw createError({ statusCode: 400, message: '无效的连接类型' })
  return { ok: true, data: sandboxController().ticket(userId, sandboxId(body?.sandboxId), stream) }
}))
