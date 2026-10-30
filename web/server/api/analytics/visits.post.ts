import { defineEventHandler } from 'h3'
import { analyticsBody } from '../../utils/analytics-body'
import { recordVisit } from '../../services/site-analytics'
export default defineEventHandler(async event => recordVisit(event, await analyticsBody(event)))
