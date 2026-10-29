import { defineEventHandler } from 'h3'
import { listChallenges } from '../../services/challenges'
export default defineEventHandler(async e => ({
  ok: true,
  data: await listChallenges(false, e.context.user?.id || null)
}))
