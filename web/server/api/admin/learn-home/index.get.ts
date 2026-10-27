import { defineEventHandler } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { getLearnHome } from '../../../services/learn-home'
export default defineEventHandler(async (e) => {
  requireAdmin(e)
  return { data: await getLearnHome() }
})
