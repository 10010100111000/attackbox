import {defineEventHandler,readBody,getRequestURL,getRequestProtocol,createError} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {labGitAction} from '../../../services/lab-git'
export default defineEventHandler(async e=>{const a=requireAdmin(e),body=await readBody(e);if(body?.token&&!['localhost','127.0.0.1','[::1]'].includes(getRequestURL(e).hostname)&&getRequestProtocol(e,{xForwardedProto:true})!=='https')throw createError({statusCode:400,message:'请使用 HTTPS 提交 Token'});return {data:await labGitAction(a.id,body)}})
