import {eq,sql} from 'drizzle-orm'
import {db} from '../db'
import {moduleSources} from '../db/schema'
import {getModuleDetail,saveModuleInTransaction} from './content'
import {fail} from './content-input'
import type {ModuleInput} from '../../shared/content'
export async function saveModuleRuntime(id:string,actor:string,body:Record<string,unknown>){
 const allowed=['lockVersion','defaultTargetTemplateId','prerequisiteModuleIds','chapters']
 if(!body||typeof body!=='object'||Array.isArray(body))fail('运行配置必须是对象')
 if(Object.keys(body).some(k=>!allowed.includes(k)))fail('运行配置接口不能修改教程内容')
 const detail=await getModuleDetail(id)
 if(!detail.source)fail('该模块尚未接管到 Git 管理',409)
 const changes=body.chapters??[]
 if(!Array.isArray(changes)||changes.some(c=>!c||typeof c!=='object')||new Set(changes.map(c=>c.chapterId)).size!==changes.length)fail('章节运行配置无效')
 for(const change of changes){if(!change||typeof change!=='object'||Object.keys(change).some(k=>!['chapterId','targetMode','targetTemplateId'].includes(k))||!detail.chapters.some(c=>c.chapterId===change.chapterId))fail('章节运行配置无效')}
 const input:ModuleInput={lockVersion:body.lockVersion as number,defaultTargetTemplateId:body.defaultTargetTemplateId as string|null|undefined,prerequisiteModuleIds:body.prerequisiteModuleIds as string[]|undefined,
  chapters:detail.chapters.map(ch=>{const change=changes.find(c=>c.chapterId===ch.chapterId);return {chapterId:ch.chapterId,title:ch.title,access:ch.access,bodyMarkdown:ch.bodyMarkdown,durationMinutes:ch.durationMinutes,targetMode:change?.targetMode??ch.targetMode,targetTemplateId:change?.targetTemplateId===undefined?ch.targetTemplateId:change.targetTemplateId}})}
 return db.transaction(async tx=>{await tx.execute(sql`select pg_advisory_xact_lock(72486211)`);const [source]=await tx.select().from(moduleSources).where(eq(moduleSources.moduleId,id));if(!source)fail('来源已变化，请重新读取',409);return saveModuleInTransaction(tx,id,actor,input)})
}
