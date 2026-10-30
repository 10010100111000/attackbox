import { defineEventHandler } from 'h3'
import { analyticsBody } from '../../utils/analytics-body'
import { recordRead } from '../../services/site-analytics'
export default defineEventHandler(async event => {
  await recordRead(event, await analyticsBody(event))
  return { accepted: true }
})
