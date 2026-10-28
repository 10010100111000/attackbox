import { defineEventHandler, readBody, setResponseHeader } from 'h3'
import { requireLearner } from '../../utils/learner'
import { submitFeedback } from '../../services/feedback'

export default defineEventHandler(async event => {
  const user = requireLearner(event, '请登录后提交反馈')
  try {
    return { data: await submitFeedback(user, await readBody(event)) }
  } catch (error: any) {
    if (error?.statusCode === 429) setResponseHeader(event, 'Retry-After', 600)
    throw error
  }
})
