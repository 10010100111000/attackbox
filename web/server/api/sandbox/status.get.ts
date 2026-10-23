import { defineEventHandler } from 'h3'
import { sandboxController } from '../../sandbox'
import { requireSandboxUser } from '../../utils/sandbox'
export default defineEventHandler(event => ({ ok: true, data: sandboxController().status(requireSandboxUser(event)) }))
