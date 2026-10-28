import {createError} from 'h3'
import {client} from '../db'
import {verifyQuizAnswer} from '../utils/quiz'
const fail=(code:number,message:string):never=>{throw createError({statusCode:code,message})}
export function challengeId(value:unknown):string{if(typeof value!=='string'||!/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(value))fail(400,'无效标识');return (value as string).toLowerCase()}
export async function listChallenges(admin=false,user:string|null=null,history=false){return client`
 select c.id,c.title,c.summary,c.category,c.difficulty,c.status,c.lock_version as "lockVersion",
 stats.last_passed_at as "lastPassedAt",
 coalesce(stats.total_stages,0) as "totalStages",
 coalesce(stats.completed_stages,0) as "completedStages"
 from challenges c
 left join (
  select s.challenge_id,
   count(*)::int as total_stages,
   count(p.stage_id)::int as completed_stages,
   max(p.passed_at) as last_passed_at
  from challenge_stages s
  left join challenge_completions p on p.stage_id=s.id and p.user_id=${user}
  group by s.challenge_id
 ) stats on stats.challenge_id=c.id
 where (${admin} or c.status='published') and (not ${history} or coalesce(stats.completed_stages,0)>0)
 order by case when ${history} then stats.last_passed_at else c.updated_at end desc,c.id`}
export async function challengeDetail(id:string,admin=false,user:string|null=null){challengeId(id);return client.begin('isolation level repeatable read read only',async tx=>{
 const [c]=await tx`select id,title,summary,body_markdown as "bodyMarkdown",category,difficulty,status,lock_version as "lockVersion",target_template_id as "targetTemplateId" from challenges where id=${id} and (${admin} or status='published')`
 if(!c)fail(404,'挑战不存在或尚未发布')
 const rows=await tx`select s.id,s.position,s.title,s.prompt_markdown as "promptMarkdown",s.hint,p.passed_at as "passedAt",case when ${admin} then s.flag else null end as flag from challenge_stages s left join challenge_completions p on p.stage_id=s.id and p.user_id=${user} where s.challenge_id=${id} order by s.position`
 let preceding=true
 const stages=rows.map(s=>{const {flag,...rest}=s;const locked=!rest.passedAt&&!preceding;preceding=preceding&&!!rest.passedAt;return {...rest,passedAt:rest.passedAt,locked,...(admin?{flag}:{})}})
 let target:any=null
 if(c.targetTemplateId){const [t]=await tx`select name,description,settings,access_instructions as "accessInstructions",enabled from target_templates where id=${c.targetTemplateId}`;if(t?.enabled)target={status:'configured',name:t.name,description:t.description,entryPort:t.settings.entryPort,accessInstructions:t.accessInstructions};else target={status:'unavailable'}}
 const completedStages=stages.filter(s=>s.passedAt).length
 if(!admin)delete c.targetTemplateId
 return {...c,stages,target,totalStages:stages.length,completedStages,isCompleted:stages.length>0&&completedStages===stages.length}
})}
export async function saveChallenge(id:string|null,body:any){
 if(id)challengeId(id)
 if(!body||typeof body.title!=='string'||!body.title.trim()||body.title.length>160||typeof body.summary!=='string'||body.summary.length>500||typeof body.bodyMarkdown!=='string'||body.bodyMarkdown.length>200000||typeof body.category!=='string'||body.category.length>60||!['beginner','intermediate','advanced'].includes(body.difficulty)||!['draft','publish'].includes(body.intent)||!Array.isArray(body.stages)||body.stages.length>30)fail(400,'请填写有效的挑战内容，阶段最多 30 个')
 const template=body.targetTemplateId==null?null:challengeId(body.targetTemplateId)
 const ids=new Set<string>()
 for(const s of body.stages){if(!s||typeof s.title!=='string'||!s.title.trim()||s.title.length>160||typeof s.promptMarkdown!=='string'||s.promptMarkdown.length>30000||typeof s.hint!=='string'||s.hint.length>4000||typeof s.flag!=='string'||s.flag.length>2000)fail(400,'请检查阶段标题、说明、提示和 Flag');if(s.id){const key=challengeId(s.id);if(ids.has(key))fail(400,'阶段不能重复');ids.add(key)}}
 if(body.intent==='publish'&&(!body.stages.length||body.stages.some((s:any)=>!s.flag.trim())||!body.bodyMarkdown.trim()))fail(400,'发布前请填写挑战正文，并为每个阶段配置 Flag')
 return client.begin(async tx=>{
  const [old]=id?await tx`select * from challenges where id=${id} for update`:[]
  if(id&&!old)fail(404,'挑战不存在')
  if(old&&old.lock_version!==body.lockVersion)fail(409,'内容已被修改，当前输入已保留，请刷新后合并修改')
  if(template){const [t]=await tx`select enabled from target_templates where id=${template} for share`;if(!t||!t.enabled&&old?.target_template_id!==template)fail(400,'请选择已启用的靶机模板')}
  const status=body.intent==='publish'?'published':'unpublished'
  const [row]=id?await tx`update challenges set title=${body.title.trim()},summary=${body.summary.trim()},body_markdown=${body.bodyMarkdown},category=${body.category.trim()},difficulty=${body.difficulty},target_template_id=${template},status=${status},lock_version=lock_version+1,updated_at=now() where id=${id} returning id`:await tx`insert into challenges(title,summary,body_markdown,category,difficulty,target_template_id,status) values(${body.title.trim()},${body.summary.trim()},${body.bodyMarkdown},${body.category.trim()},${body.difficulty},${template},${status}) returning id`
  const oldStages=await tx`select id,position from challenge_stages where challenge_id=${row!.id}`
  for(const key of ids)if(!oldStages.some(s=>s.id===key))fail(409,'阶段不属于此挑战')
  const removedIds=oldStages.filter(s=>!ids.has(s.id)).map(s=>s.id)
  if(removedIds.length)await tx`delete from challenge_stages where id in ${tx(removedIds)}`
  const newStages:Record<string,unknown>[]=[]
  for(const [index,s] of body.stages.entries()){const values={title:s.title.trim(),prompt_markdown:s.promptMarkdown,hint:s.hint,flag:s.flag.trim(),position:index+1};if(s.id){await tx`update challenge_stages set ${tx(values)} where id=${challengeId(s.id)}`}else{newStages.push({...values,challenge_id:row!.id})}}
  if(newStages.length)await tx`insert into challenge_stages ${tx(newStages)}`
  return {id:row!.id}
 })
}
export async function attemptChallenge(id:string,user:string,body:any){id=challengeId(id);const stageId=challengeId(body?.stageId);if(typeof body?.flag!=='string'||!body.flag.trim()||body.flag.length>2000)fail(400,'请输入 Flag（最多 2000 字符）')
 // Return rejection values from the transaction so failed attempts remain counted.
 const result=await client.begin(async tx=>{
  const [c]=await tx`select id from challenges where id=${id} and status='published' for share`;if(!c)fail(404,'挑战不存在或尚未发布')
  await tx`select pg_advisory_xact_lock(893421,hashtext(${user+':'+id}))`
  const stages=await tx`select s.id,s.flag,p.passed_at from challenge_stages s left join challenge_completions p on p.stage_id=s.id and p.user_id=${user} where s.challenge_id=${id} order by s.position`
  const index=stages.findIndex(s=>s.id===stageId);if(index<0)fail(404,'阶段不存在')
  if(stages[index]!.passed_at)return {correct:true,alreadyPassed:true}
  if(stages.slice(0,index).some(s=>!s.passed_at))fail(403,'请先完成前面的阶段')
  const [limit]=await tx`insert into challenge_attempt_limits(user_id,challenge_id,attempts) values(${user},${id},1) on conflict(user_id,challenge_id) do update set attempts=case when challenge_attempt_limits.window_at<now()-interval '1 minute' then 1 else challenge_attempt_limits.attempts+1 end,window_at=case when challenge_attempt_limits.window_at<now()-interval '1 minute' then now() else challenge_attempt_limits.window_at end returning attempts`
  if(limit!.attempts>20)return {limited:true}
  const correct=verifyQuizAnswer(body.flag,stages[index]!.flag,'trim')
  if(correct)await tx`insert into challenge_completions(user_id,stage_id) values(${user},${stageId}) on conflict do nothing`
  return {correct}
 })
 if('limited'in result)fail(429,'提交过于频繁，请稍后再试')
 return result
}
export async function deleteChallenge(id:string,version:unknown){challengeId(id);if(!Number.isInteger(version))fail(400,'缺少修改标识');const rows=await client`delete from challenges where id=${id} and lock_version=${Number(version)} returning id`;if(!rows.length)fail(409,'挑战已变化，请刷新列表')}
