import { defineEventHandler } from 'h3'
import { requireLearner } from '../../utils/learner'
import { listChallenges } from '../../services/challenges'
export default defineEventHandler(async e => ({
  ok: true,
  data: await listChallenges(false, requireLearner(e, '请登录后查看挑战记录'), true)
}))
