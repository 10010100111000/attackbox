import {defineEventHandler,readBody,getRequestURL,getRequestProtocol,createError} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {saveComposeTemplate} from '../../../services/compose-templates'
export default defineEventHandler(async e=>{const a=requireAdmin(e),body=await readBody(e);if(Object.keys(body?.variables||{}).length&&!['localhost','127.0.0.1','[::1]'].includes(getRequestURL(e).hostname)&&getRequestProtocol(e,{xForwardedProto:true})!=='https')throw createError({statusCode:400,message:'请使用 HTTPS 提交部署变量'});return {data:await saveComposeTemplate(a.id,null,body)}})
