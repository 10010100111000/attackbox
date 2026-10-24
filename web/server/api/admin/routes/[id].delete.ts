import {defineEventHandler,getRouterParam,readBody} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {deleteRoute} from '../../../services/routes'
export default defineEventHandler(async e=>{requireAdmin(e);return {ok:true,data:await deleteRoute(getRouterParam(e,'id')!,(await readBody(e))?.lockVersion)}})
