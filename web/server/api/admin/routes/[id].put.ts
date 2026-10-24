import {defineEventHandler,getRouterParam,readBody} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {saveRoute} from '../../../services/routes'
export default defineEventHandler(async e=>{requireAdmin(e);return {ok:true,data:await saveRoute(getRouterParam(e,'id')!,await readBody(e))}})
