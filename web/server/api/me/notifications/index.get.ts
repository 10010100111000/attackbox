import { defineEventHandler, getQuery } from 'h3'
import { requireLearner } from '../../../utils/learner'
import { listNotifications } from '../../../services/notifications'
export default defineEventHandler(async (e) => ({
  data: await listNotifications(requireLearner(e), getQuery(e).page),
}))
