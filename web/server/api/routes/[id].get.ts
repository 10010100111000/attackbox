import {defineEventHandler,getRouterParam} from 'h3'
import {routeDetail} from '../../services/routes'
export default defineEventHandler(async e=>({ok:true,data:await routeDetail(getRouterParam(e,'id')!,false,e.context.user?.id||null)}))
