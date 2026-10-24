import { createError } from 'h3'
import { client } from '../db'
import { userId } from './admin-users'
const fail = (statusCode:number,message:string):never => { throw createError({statusCode,message}) }
export async function routeList(admin=false) {
 return client`select r.id,r.title,r.summary,r.status,r.lock_version as "lockVersion",
 (select count(*)::int from learning_route_modules rm join modules m on m.id=rm.module_id where rm.route_id=r.id and (${admin} or m.status='published')) as "moduleCount"
 from learning_routes r where ${admin} or (r.status='published' and exists(select 1 from learning_route_modules rm join modules m on m.id=rm.module_id where rm.route_id=r.id and m.status='published')) order by r.updated_at desc,r.id`
}
export async function routeDetail(id:string,admin=false,learner:string|null=null) {
 userId(id)
 return client.begin('isolation level repeatable read read only',async tx=>{
  const [route]=await tx`select id,title,summary,status,lock_version as "lockVersion" from learning_routes where id=${id} and (${admin} or status='published')`
  if(!route) return fail(404,'路线不存在')
  const modules=await tx`select m.id,m.title,m.slug,m.summary,m.difficulty,m.status,
   (select count(*)::int from chapters c where c.module_id=m.id) as "chapterCount",
   (select coalesce(sum(c.duration_minutes),0)::int from chapters c where c.module_id=m.id) as "durationMinutes",
   (select count(*)::int from chapters c join chapter_progress p on p.chapter_id=c.id where c.module_id=m.id and p.user_id=${learner} and p.completed_at is not null) as "completedChapters"
   from learning_route_modules rm join modules m on m.id=rm.module_id where rm.route_id=${id} and (${admin} or m.status='published') order by rm.position`
  if(!admin&&!modules.length)return fail(404,'路线暂不可用')
  return {...route,modules,moduleCount:modules.length,chapterCount:modules.reduce((n,m)=>n+m.chapterCount,0),durationMinutes:modules.reduce((n,m)=>n+m.durationMinutes,0),completedChapters:modules.reduce((n,m)=>n+m.completedChapters,0)}
 })
}
export async function saveRoute(id:string|null,body:any) {
 if(id)userId(id)
 if(!body||typeof body.title!=='string'||!body.title.trim()||body.title.trim().length>160||typeof body.summary!=='string'||body.summary.length>500||!['draft','publish'].includes(body.intent)||!Array.isArray(body.moduleIds)||body.moduleIds.length>100)fail(400,'请填写路线名称、简介和模块列表')
 const ids:string[]=body.moduleIds.map((id:unknown)=>userId(id).toLowerCase())
 if(new Set(ids).size!==ids.length)fail(400,'同一路线不能重复添加模块')
 if(body.intent==='publish'&&!ids.length)fail(400,'请先添加学习模块')
 return client.begin(async tx=>{
  if(id){const [old]=await tx`select lock_version from learning_routes where id=${id} for update`;if(!old)fail(404,'路线不存在');if(old.lock_version!==body.lockVersion)fail(409,'路线已被修改，请刷新后再保存')}
  if(ids.length){const found=await tx`select id,status from modules where id in ${tx(ids)} for share`;if(found.length!==ids.length)fail(400,'部分模块不存在');if(body.intent==='publish'&&found.some(m=>m.status!=='published'))fail(409,'发布前请移除或发布尚未公开的模块')}
  const status=body.intent==='publish'?'published':'unpublished'
  const [row]=id?await tx`update learning_routes set title=${body.title.trim()},summary=${body.summary.trim()},status=${status},lock_version=lock_version+1,updated_at=now() where id=${id} returning id`:await tx`insert into learning_routes(title,summary,status) values(${body.title.trim()},${body.summary.trim()},${status}) returning id`
  await tx`delete from learning_route_modules where route_id=${row!.id}`
  for(const [index,moduleId] of ids.entries())await tx`insert into learning_route_modules(route_id,module_id,position) values(${row!.id},${moduleId},${index+1})`
  return {id:row!.id}
 })
}
export async function deleteRoute(id:string,version:unknown){userId(id);if(!Number.isSafeInteger(version))fail(400,'缺少路线版本');const rows=await client`delete from learning_routes where id=${id} and lock_version=${Number(version)} returning id`;if(!rows.length)fail(409,'路线已变化或已删除，请刷新列表')}
