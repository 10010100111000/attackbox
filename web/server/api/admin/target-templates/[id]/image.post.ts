import {defineEventHandler,readBody,getRouterParam,createError} from 'h3'
import {randomBytes} from 'node:crypto'
import {requireAdmin} from '../../../../utils/admin'
import {getTargetTemplate} from '../../../../services/target-templates'
import {sandboxHub} from '../../../../sandbox'
export default defineEventHandler(async event => {
 const admin=requireAdmin(event)
 const template=await getTargetTemplate(getRouterParam(event,'id')!)
 const body=await readBody(event)
 if (!['status','prepare'].includes(body?.action)) throw createError({statusCode:400,message:'无效操作'})
 const hub=sandboxHub()
 const nodes=hub.snapshot().nodes.filter(n=>n.state==='online'&&n.bootId&&hub.targetNodes.has(n.id))
 const data=await Promise.all(nodes.map(async node=>{
  try {
   const result=await hub.targetRpc(node.id,node.bootId!,body.action==='prepare'?'target_image_prepare':'target_image_status',{target_id:'tgt_'+randomBytes(16).toString('hex'),owner_id:admin.id,image:template.image},15000)
   return {nodeId:node.id,state:result.success?result.image_state:'failed',error:result.error}
  } catch { return {nodeId:node.id,state:'unknown',error:'节点未响应，请重试'} }
 }))
 return {data}
})
