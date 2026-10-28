import {defineEventHandler,getRouterParam,getRequestHeaders,createError,setResponseHeader} from 'h3'
import {authorizeWorker} from '../../../sandbox/config'
import {sandboxHub} from '../../../sandbox'
import {bundleRecord} from '../../../services/compose-templates'
import {readBundleFiles} from '../../../labs/storage'
export default defineEventHandler(async e=>{
 const headers=new Headers();for(const [k,v]of Object.entries(getRequestHeaders(e)))if(typeof v==='string')headers.set(k,v)
 const denied=authorizeWorker(sandboxHub().config,headers);if(denied)throw createError({statusCode:denied,message:'Worker 身份验证失败'})
 setResponseHeader(e,'X-Content-Type-Options','nosniff')
 const b=await bundleRecord(getRouterParam(e,'id')!),files=await readBundleFiles(b.id,b.manifest)
 return {id:b.id,hash:b.hash,files:Object.fromEntries([...files].map(([k,v])=>[k,v.toString('base64')]))}
})
