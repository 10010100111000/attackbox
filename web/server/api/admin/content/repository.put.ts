import {defineEventHandler,readBody,getRequestHeader,getRequestURL,createError} from 'h3'
import {requireAdmin} from '../../../utils/admin'
import {saveContentRepository} from '../../../services/content-repository'
export default defineEventHandler(async e=>{requireAdmin(e);const body=await readBody(e);const url=getRequestURL(e);if(body?.token&&url.protocol!=='https:'&&getRequestHeader(e,'x-forwarded-proto')!=='https'&&!['localhost','127.0.0.1','[::1]'].includes(url.hostname))throw createError({statusCode:400,message:'请通过 HTTPS 设置仓库 Token'});return {data:await saveContentRepository(body||{})}})
