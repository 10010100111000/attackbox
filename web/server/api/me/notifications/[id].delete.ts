import { defineEventHandler, getRouterParam } from 'h3'
import { requireLearner } from '../../../utils/learner'
import { dismissNotification } from '../../../services/notifications'
export default defineEventHandler(e=>dismissNotification(requireLearner(e),getRouterParam(e,'id')))
