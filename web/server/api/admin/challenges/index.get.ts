import { defineEventHandler } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { listChallenges } from '../../../services/challenges'
export default defineEventHandler(async e => {
  requireAdmin(e)
  return { ok: true, data: await listChallenges(true) }
})
