import { defineEventHandler, readBody } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { saveRuntimePolicy } from '../../../services/runtime-policy'
export default defineEventHandler(async (e) => {
  const actor = requireAdmin(e)
  return { data: await saveRuntimePolicy(await readBody(e), actor.id) }
})
