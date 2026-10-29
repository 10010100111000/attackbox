import { defineEventHandler, readBody, createError } from 'h3'
import { requireLearner } from '../../utils/learner'
import { targetOperation } from '../../services/target-runtime'
export default defineEventHandler(async event => {
  const user = requireLearner(event),
    body = await readBody(event)
  if (!body || !['start', 'stop'].includes(body.action))
    throw createError({ statusCode: 400, message: '操作无效' })
  return targetOperation(user, body, body.action)
})
