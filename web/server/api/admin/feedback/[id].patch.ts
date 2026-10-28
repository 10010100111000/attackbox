import { defineEventHandler, getRouterParam, readBody } from 'h3'
import { requireAdmin } from '../../../utils/admin'
import { updateFeedbackStatus } from '../../../services/feedback'

export default defineEventHandler(async event => {
  requireAdmin(event)
  const body = await readBody(event)
  return { data: await updateFeedbackStatus(getRouterParam(event, 'id'), body?.status) }
})
