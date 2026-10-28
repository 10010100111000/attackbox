import {and,eq} from 'drizzle-orm'
import {db} from '../db'
import {targetInstances} from '../db/schema'
import {sandboxHub} from '../sandbox'
import {targetReplyState} from '../../shared/target-runtime'
type Row=typeof targetInstances.$inferSelect
const inFlightInspects=new Map<string,Promise<any>>()
export async function composeTargetOperation(row:Row,action:'start'|'inspect'|'stop'){
 const hub=sandboxHub(),node=hub.snapshot().nodes.find(n=>n.id===row.nodeId&&n.state==='online'),view=(r:Row)=>({id:r.id,expiresAt:r.expiresAt,state:r.state,instance:r.result,updatedAt:r.updatedAt})
 if(!node?.bootId||!hub.composeNodes.has(node.id))return {...view(row),error:'节点离线或 Compose 能力不可用，状态待确认'}
 if(action==='inspect'){
  const pending=inFlightInspects.get(row.id)
  if(pending)return pending
 }
 const execute=async()=>{
  let revision=row.revision
  if(action!=='inspect'){const [claimed]=await db.update(targetInstances).set({revision:row.revision+1,...(action==='stop'?{state:'stopping'}:{})}).where(and(eq(targetInstances.id,row.id),eq(targetInstances.revision,row.revision))).returning();if(!claimed)return {...view(row),error:'环境正在处理其他操作'};revision=claimed.revision}
  try{
   const result=await hub.composeRpc(row.nodeId,node.bootId,action==='start'?'compose_start':action==='stop'?'compose_stop':'compose_inspect',{target_id:row.id,owner_id:row.userId,deployment_id:row.deploymentId!,expires_at:row.expiresAt.toISOString()})
   const state=targetReplyState(row.state,action,result)
   const nextResult=result.instance?{...result.instance}:row.result
   const unchanged=action==='inspect'&&state===row.state&&row.bootId===node.bootId&&JSON.stringify(nextResult)===JSON.stringify(row.result)
   if(unchanged){
    return {...view(row),...(!result.success?{error:'节点尚未确认操作结果，可释放环境后重新启动；仍失败时请联系管理员检查部署'}:{})}
   }
   const [updated]=await db.update(targetInstances).set({state,result:nextResult,bootId:node.bootId,updatedAt:new Date()}).where(and(eq(targetInstances.id,row.id),eq(targetInstances.revision,revision))).returning()
   if(!updated){const [latest]=await db.select().from(targetInstances).where(and(eq(targetInstances.userId,row.userId),eq(targetInstances.scope,row.scope)));if(latest)return view(latest);return {...view(row),state:'unconfirmed',error:'环境状态已变化，请重新打开页面确认'}}
   return {...view(updated||row),...(!result.success?{error:'节点尚未确认操作结果，可释放环境后重新启动；仍失败时请联系管理员检查部署'}:{})}
  }catch{return {...view(row),error:'Worker 结果待确认，请勿重复创建环境'}}
 }
 if(action!=='inspect')return execute()
 const p=execute().finally(()=>inFlightInspects.delete(row.id))
 inFlightInspects.set(row.id,p)
 return p
}
