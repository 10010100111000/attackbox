import { defineEventHandler, readBody } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { saveLearnHome } from '../../../services/learn-home'
export default defineEventHandler(async (e) => {
  const actor = requireAdmin(e)
  return { data: await saveLearnHome(await readBody(e), actor.id) }
})
