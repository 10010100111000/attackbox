import {defineEventHandler,getRouterParam} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {challengeDetail} from '../../../services/challenges'
export default defineEventHandler(async e=>{requireAdmin(e);return {ok:true,data:await challengeDetail(getRouterParam(e,'id')!,true)}})
