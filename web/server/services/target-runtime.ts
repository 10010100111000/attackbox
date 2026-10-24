import {randomBytes} from 'node:crypto'
import {and,eq,sql} from 'drizzle-orm'
import {createError} from 'h3'
import {db} from '../db'
import {targetInstances,targetTemplates,chapters,modules,challenges} from '../db/schema'
import {chapterContext} from './learning'
import {sandboxHub,sandboxController} from '../sandbox'

export async function targetScope(user:string,input:Record<string,unknown>){
 return db.transaction(async tx=>{
  if(input.kind==='challenge'){
   if(typeof input.challengeId!=='string'||!/^[a-f0-9-]{36}$/.test(input.challengeId))throw createError({statusCode:400,message:'挑战参数无效'})
   const [c]=await tx.select().from(challenges).where(and(eq(challenges.id,input.challengeId),eq(challenges.status,'published')))
   if(!c)throw createError({statusCode:404,message:'挑战不可用'})
   return {scope:'challenge:'+c.id,templateId:c.targetTemplateId}
  }
  if(input.kind!=='chapter'||typeof input.moduleSlug!=='string'||typeof input.chapterSlug!=='string')throw createError({statusCode:400,message:'章节参数无效'})
  const {chapter}=await chapterContext(tx,input.moduleSlug,input.chapterSlug,user)
  const [c]=await tx.select({mode:chapters.targetMode,template:chapters.targetTemplateId,moduleTemplate:modules.defaultTargetTemplateId,module:modules.id}).from(chapters).innerJoin(modules,eq(modules.id,chapters.moduleId)).where(eq(chapters.id,chapter.id))
  return {scope:c!.mode==='custom'?'chapter:'+chapter.id:'module:'+c!.module,templateId:c!.mode==='none'?null:c!.mode==='custom'?c!.template:c!.moduleTemplate}
 })
}
function view(row:typeof targetInstances.$inferSelect){return {id:row.id,state:row.state,instance:row.result,updatedAt:row.updatedAt}}
export async function targetOperation(user:string,input:Record<string,unknown>,action:'start'|'inspect'|'stop'){
 const hub=sandboxHub()
 const scope=action==='stop'?null:await targetScope(user,input)
 const row=await db.transaction(async tx=>{
  await tx.execute(sql`select pg_advisory_xact_lock(19842,hashtext(${user}))`)
  const [existing]=await tx.select().from(targetInstances).where(and(eq(targetInstances.userId,user),action==='stop'?eq(targetInstances.id,String(input.id||'')):eq(targetInstances.scope,scope!.scope)))
  if(action!=='start')return existing
  if(!scope?.templateId)throw createError({statusCode:409,message:'未关联靶机模板'})
  const [template]=await tx.select().from(targetTemplates).where(and(eq(targetTemplates.id,scope.templateId),eq(targetTemplates.enabled,true)))
  if(!template)throw createError({statusCode:409,message:'靶机模板已停用'})
  if(existing&&existing.state!=='stopped')return existing
  if(template.ports.length>16)throw createError({statusCode:409,message:'当前 Worker 最多支持 16 个服务端口'})
  const nodes=hub.snapshot().nodes.filter(n=>n.state==='online'&&!n.settings.draining&&n.bootId&&hub.targetNodes.has(n.id))
  const attacker=sandboxController().status(user).instance
  const eligible=attacker?nodes.filter(n=>n.id===attacker.nodeId):hub.snapshot().nodes.filter(n=>n.state==='online').length===1?nodes:[]
  if(eligible.length!==1)throw createError({statusCode:503,message:'需要一个可用的 Docker 节点，且与攻击环境位于同一节点'})
  const node=eligible[0]!
  const values={id:'tgt_'+randomBytes(16).toString('hex'),userId:user,scope:scope.scope,nodeId:node.id,bootId:node.bootId!,state:'starting',image:template.image,ports:template.ports,result:null,revision:(existing?.revision||0)+1,updatedAt:new Date()}
  if(existing){await tx.delete(targetInstances).where(eq(targetInstances.id,existing.id))}
  const [created]=await tx.insert(targetInstances).values(values).returning();return created
 })
 if(!row)return null
 if(action==='inspect'&&row.state==='stopped')return view(row)
 if(action==='start'&&row.state==='stopping')throw createError({statusCode:409,message:'正在停止，请先刷新状态'})
 const node=hub.snapshot().nodes.find(n=>n.id===row.nodeId&&n.state==='online')
 if(!node?.bootId)return {...view(row),error:'节点离线，实例状态待确认'}
 const claimed=await db.update(targetInstances).set({revision:row.revision+1,...(action==='stop'?{state:'stopping'}:{})}).where(and(eq(targetInstances.id,row.id),eq(targetInstances.revision,row.revision))).returning()
 if(!claimed.length)throw createError({statusCode:409,message:'环境正在被其他请求操作，请刷新后重试'})
 try{
  const result=await hub.targetRpc(row.nodeId,node.bootId,action==='start'?'target_start':action==='stop'?'target_stop':'target_inspect',{target_id:row.id,owner_id:user,...(action==='start'?{image:row.image,ports:row.ports}:{})})
  const state=result.instance?.state|| (action==='stop'?'stopping':row.state)
  const [updated]=await db.update(targetInstances).set({state,result:result.instance?{...result.instance}:row.result,bootId:node.bootId,updatedAt:new Date()}).where(and(eq(targetInstances.id,row.id),eq(targetInstances.revision,row.revision+1))).returning()
  return {...view(updated||row),...(!result.success?{error:result.error||'Worker 操作失败'}:{})}
 }catch{return {...view(row),error:'Worker 响应未确认，请刷新状态；不要重复创建实例'}}
}
