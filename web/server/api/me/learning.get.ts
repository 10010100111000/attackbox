import { defineEventHandler } from 'h3'
import { requireLearner } from '../../utils/learner'
import { learningHistory } from '../../services/learning'
export default defineEventHandler(async event => ({ ok: true, data: await learningHistory(requireLearner(event)) }))
