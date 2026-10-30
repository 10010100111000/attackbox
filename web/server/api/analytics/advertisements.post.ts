import { defineEventHandler } from 'h3'
import { analyticsBody } from '../../utils/analytics-body'
import { recordAdEvent } from '../../services/site-analytics'
export default defineEventHandler(async event => recordAdEvent(event, await analyticsBody(event)))
