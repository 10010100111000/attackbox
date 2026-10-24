import {defineEventHandler,getRouterParam} from 'h3'
import {challengeDetail} from '../../services/challenges'
export default defineEventHandler(async e=>({ok:true,data:await challengeDetail(getRouterParam(e,'id')!,false,e.context.user?.id||null)}))
