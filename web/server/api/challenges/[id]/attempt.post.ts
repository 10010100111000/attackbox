import {defineEventHandler,getRouterParam,readBody} from 'h3'
import {requireLearner} from '../../../utils/learner'
import {attemptChallenge} from '../../../services/challenges'
export default defineEventHandler(async e=>({ok:true,data:await attemptChallenge(getRouterParam(e,'id')!,requireLearner(e,'请登录后提交 Flag'),await readBody(e))}))
