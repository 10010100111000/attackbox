import {defineEventHandler,readBody,getRouterParam,createError,setResponseHeader} from 'h3'
import {requireAdmin} from '../../../../utils/admin'
import {preparationAction} from '../../../../services/compose-preparations'
export default defineEventHandler(async e=>{requireAdmin(e);setResponseHeader(e,'Cache-Control','private, no-store');const body=await readBody(e);if(!['status','prepare','cancel','rebuild'].includes(body?.action)||typeof body.nodeId!=='string')throw createError({statusCode:400,message:'请选择节点和操作'});return {data:await preparationAction(getRouterParam(e,'id')!,body.nodeId,body.action,body.expectedAttemptId)}})
