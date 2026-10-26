import { defineEventHandler, getRouterParam } from 'h3'
import { requireLearner } from '../../../../utils/learner'
import { readNotification } from '../../../../services/notifications'
export default defineEventHandler((e) =>
  readNotification(requireLearner(e), getRouterParam(e, 'id')),
)
