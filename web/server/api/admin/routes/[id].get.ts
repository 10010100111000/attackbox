import {defineEventHandler,getRouterParam} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {routeDetail} from '../../../services/routes'
export default defineEventHandler(async e=>{requireAdmin(e);return {ok:true,data:await routeDetail(getRouterParam(e,'id')!,true)}})
