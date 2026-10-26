import { defineEventHandler } from 'h3'
import { requireLearner } from '../../../utils/learner'
import { unreadNotifications } from '../../../services/notifications'
export default defineEventHandler(async (e) => ({
  count: await unreadNotifications(requireLearner(e)),
}))
