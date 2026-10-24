import {defineEventHandler,getQuery} from 'h3'
import {requireLearner} from '../../utils/learner'
import {targetOperation} from '../../services/target-runtime'
export default defineEventHandler(event=>targetOperation(requireLearner(event),getQuery(event),'inspect'))
