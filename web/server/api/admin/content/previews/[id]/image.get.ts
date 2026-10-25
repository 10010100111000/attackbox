import {defineEventHandler,getRouterParam,getQuery,setResponseHeaders} from 'h3'
import {requireAdmin} from '../../../../../utils/admin'
import {contentPreviewImage} from '../../../../../services/content-sync'
export default defineEventHandler(async e=>{const admin=requireAdmin(e),query=getQuery(e);const image=await contentPreviewImage(getRouterParam(e,'id')!,admin.id,String(query.source||''),String(query.href||''));setResponseHeaders(e,{'Content-Type':image.mediaType,'X-Content-Type-Options':'nosniff','Cache-Control':'private, no-store'});return image.bytes})
