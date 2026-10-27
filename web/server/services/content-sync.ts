import {randomUUID} from 'node:crypto'
import path from 'node:path'
import {readFile,mkdir,lstat} from 'node:fs/promises'
import {and,eq,sql,inArray} from 'drizzle-orm'
import {createError} from 'h3'
import MarkdownIt from 'markdown-it'
import {db} from '../db'
import {modules,chapters,chapterQuestions,contentCategories,moduleSources,chapterSources,questionSources,contentPreviews,contentRepository,assets,chapterAssets,moduleImages} from '../db/schema'
import type {ContentBindings} from '../db/schema/content-sync'
import {checkContentDirectory} from '../content/read-directory'
import {learnerContentPreview} from '../content/parse-module'
import type {ParsedModule} from '../content/parse-module'
import {hash,previewDirectory,discardPreview,storeImage,writeSnapshotFile} from '../content/storage'
import {pruneUnusedCategories,saveModuleInTransaction} from './content'
import {slugFor} from './content-input'
import type {ModuleInput} from '../../shared/content'

type Tx=Parameters<Parameters<typeof db.transaction>[0]>[0]
const emptyBindings=():ContentBindings=>({modules:{},chapters:{},questions:{}})
export const bindingKey=(...parts:string[])=>JSON.stringify(parts)
function fail(message:string,statusCode=409):never{throw createError({statusCode,message})}
function findCategory(categories:typeof contentCategories.$inferSelect[],key:string|null){
 if(!key)return null
 return categories.find(c=>c.slug===key||c.name===key)||null
}
async function ensureCategoryId(tx:Tx,categories:typeof contentCategories.$inferSelect[],key:string|null){
 if(!key)return null
 const existing=findCategory(categories,key)
 if(existing)return existing.id
 const candidate=/^[a-z0-9]+(-[a-z0-9]+)*$/.test(key)&&key.length<=100&&!categories.some(c=>c.slug===key)?key:slugFor(undefined,'category')
 const [created]=await tx.insert(contentCategories).values({name:key,slug:candidate}).returning()
 categories.push(created!)
 return created!.id
}
async function state(tx:Tx){
 const [mods,chs,qs,sources,cs,questionLinks,categories]=await Promise.all([tx.select().from(modules),tx.select().from(chapters),tx.select().from(chapterQuestions),tx.select().from(moduleSources),tx.select().from(chapterSources),tx.select().from(questionSources),tx.select().from(contentCategories)])
 const baseline=hash(JSON.stringify({modules:mods.map(m=>[m.id,m.lockVersion]).sort(),sources:sources.map(s=>[s.moduleId,s.origin,s.sourcePath,s.sourceHash]).sort(),categories:categories.map(c=>[c.id,c.slug,c.name]).sort()}))
 return {mods,chs,qs,sources,cs,questionLinks,categories,baseline}
}
async function snapshot(id:string){
 const root=previewDirectory(id),checked=await checkContentDirectory(root)
 const images=new Map<string,{bytes:Buffer;hash:string;mediaType:string}>()
 const blobs=new Map<string,{bytes:Buffer;hash:string;mediaType:string}>()
 const types:Record<string,string>={'.png':'image/png','.jpg':'image/jpeg','.jpeg':'image/jpeg','.webp':'image/webp','.gif':'image/gif','.avif':'image/avif'}
 for(const item of checked){
  const fingerprints=[]
  for(const image of item.images){let blob=blobs.get(image.repositoryPath);if(!blob){const bytes=await readFile(path.join(root,image.repositoryPath));blob={bytes,hash:hash(bytes),mediaType:types[path.extname(image.repositoryPath).toLowerCase()]!};blobs.set(image.repositoryPath,blob)}images.set(bindingKey(item.module.sourcePath,image.href),blob);fingerprints.push([image.href,blob.hash])}
  item.module.sourceHash=hash(JSON.stringify([item.module.sourceHash,fingerprints.sort()]))
 }
 return {modules:checked.map(c=>c.module),images,manifest:hash(JSON.stringify(checked.map(c=>[c.module.sourcePath,c.module.sourceHash])))}
}
function validateBindings(value:unknown):ContentBindings{
 const result=emptyBindings()
 if(value===undefined)return result
 if(!value||typeof value!=='object'||Array.isArray(value))fail('关联配置无效',400)
 for(const type of Object.keys(value)){if(!['modules','chapters','questions'].includes(type))fail('关联类型无效',400)}
 for(const type of ['modules','chapters','questions'] as const){const entries=(value as any)[type]||{};if(typeof entries!=='object'||Array.isArray(entries))fail('关联配置无效',400);for(const [key,id] of Object.entries(entries)){if(typeof id!=='string'||!/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(id)||key.length>30000)fail('关联标识无效',400);Object.defineProperty(result[type],key,{value:id,enumerable:true})}}
 return result
}
function plan(parsed:ParsedModule[],origin:string,bindings:ContentBindings,current:Awaited<ReturnType<typeof state>>){
 const conflicts:string[]=[],removals:string[]=[],usedModules=new Set<string>(),usedChapters=new Set<string>(),usedQuestions=new Set<string>()
 const output=parsed.map(module=>{
  const explicit=bindings.modules[module.sourcePath]
  const source=current.sources.find(s=>s.origin===origin&&s.sourcePath===module.sourcePath)
  const old=current.mods.find(m=>m.id===(explicit||source?.moduleId))
  if(explicit&&!old)fail('关联模块不存在')
  if(old&&usedModules.has(old.id))fail('一个旧模块不能关联多个文件')
  if(old)usedModules.add(old.id)
  const previous=old?current.sources.find(s=>s.moduleId===old.id):undefined
  const category=findCategory(current.categories,module.categorySlug)
  let hasRemovals=false
  const plannedChapters=module.chapters.map(ch=>{
   const key=bindingKey(module.sourcePath,ch.matchKey),chosen=bindings.chapters[key]
   const link=old?current.cs.find(c=>c.moduleId===old.id&&c.matchKey===ch.matchKey):undefined
   const existing=current.chs.find(c=>c.id===(chosen||link?.chapterId))
   if(chosen&&(!existing||existing.moduleId!==old?.id))fail('章节关联不属于当前模块')
   if(existing&&usedChapters.has(existing.id))fail('一个旧章节不能关联多个章节')
   if(existing)usedChapters.add(existing.id)
   const plannedQuestions=ch.questions.map(q=>{
    const qkey=bindingKey(module.sourcePath,ch.matchKey,q.matchKey),qid=bindings.questions[qkey]
    const qlink=existing?current.questionLinks.find(l=>l.chapterId===existing.id&&l.matchKey===q.matchKey):undefined
    const oldQ=current.qs.find(p=>p.id===(qid||qlink?.questionId))
    if(qid&&(!oldQ||oldQ.chapterId!==existing?.id))fail('题目关联不属于当前章节')
    if(oldQ&&usedQuestions.has(oldQ.id))fail('一个旧题目不能重复关联')
    if(oldQ)usedQuestions.add(oldQ.id)
    return {parsed:q,existing:oldQ,key:qkey}
   })
   if(existing)for(const oldQ of current.qs.filter(q=>q.chapterId===existing.id))if(!plannedQuestions.some(q=>q.existing?.id===oldQ.id)){hasRemovals=true;removals.push(`${module.sourcePath} / ${ch.title}：将删除未关联题目“${oldQ.promptMarkdown.slice(0,80)}”（若为改名请先在下方关联）`)}
   return {parsed:ch,existing,key,questions:plannedQuestions}
  })
  if(old)for(const removed of current.chs.filter(c=>c.moduleId===old.id))if(!plannedChapters.some(c=>c.existing?.id===removed.id)){hasRemovals=true;removals.push(`${module.sourcePath}：将删除未关联章节“${removed.title}”（若为改名请先在下方关联）`)}
  const categoryChanged=module.categorySlug?(!category||old?.categoryId!==category.id):!!old?.categoryId
  const changed=!old||!previous||previous.sourceHash!==module.sourceHash||previous.sourcePath!==module.sourcePath||previous.origin!==origin||categoryChanged||hasRemovals||!!explicit||plannedChapters.some(c=>!!bindings.chapters[c.key]||c.questions.some(q=>!!bindings.questions[q.key]))
  return {parsed:module,existing:old,categoryId:category?.id||null,chapters:plannedChapters,changed}
 })
 for(const entry of output){const occupied=current.sources.find(s=>s.origin===origin&&s.sourcePath===entry.parsed.sourcePath);if(occupied&&occupied.moduleId!==entry.existing?.id&&!usedModules.has(occupied.moduleId))conflicts.push(`${entry.parsed.sourcePath}：该路径已有关联模块，不能直接覆盖为另一个模块`)}
 return {modules:output,conflicts,removals,missingModules:current.sources.filter(s=>!usedModules.has(s.moduleId)).map(s=>({id:s.moduleId,path:s.sourcePath,title:current.mods.find(m=>m.id===s.moduleId)?.title||s.sourcePath}))}
}
async function previewRecord(id:string,actor:string,tx:Tx,writing=false){if(!/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(id))fail('预览标识无效',400);const query=tx.select().from(contentPreviews).where(and(eq(contentPreviews.id,id),eq(contentPreviews.actorId,actor)));const [p]=await (writing?query.for('update'):query);if(!p)fail('预览不存在',404);return p}
async function publicPlan(id:string,actor:string){
 const p=await db.transaction(tx=>previewRecord(id,actor,tx))
 if(p.result)return {id,commit:p.commit,applied:true,result:p.result}
 if(p.expiresAt.getTime()<Date.now())fail('预览已过期，请重新拉取')
 const files=await snapshot(id)
 if(files.manifest!==p.manifest)fail('预览文件已变化，请重新拉取')
 return db.transaction(async tx=>{
  const [config]=p.configVersion===null?[]:await tx.select().from(contentRepository).where(eq(contentRepository.id,1))
  const current=await state(tx),report=plan(files.modules,p.origin,p.bindings,current)
  return {id,applied:false,commit:p.commit,expiresAt:p.expiresAt,stale:current.baseline!==p.baseline||(p.configVersion!==null&&config?.version!==p.configVersion),bindings:p.bindings,conflicts:report.conflicts,removals:report.removals,missingModules:report.missingModules,
   candidates:current.mods.map(m=>({id:m.id,title:m.title,chapters:current.chs.filter(c=>c.moduleId===m.id).map(c=>({id:c.id,title:c.title,questions:current.qs.filter(q=>q.chapterId===c.id).map(q=>({id:q.id,prompt:q.promptMarkdown}))}))})),
   modules:report.modules.map(m=>({path:m.parsed.sourcePath,id:m.existing?.id||null,changed:m.changed,preview:{...learnerContentPreview(m.parsed),imageUrls:Object.fromEntries(m.parsed.images.map(image=>[image.href,`/api/admin/content/previews/${id}/image?source=${encodeURIComponent(m.parsed.sourcePath)}&href=${encodeURIComponent(image.href)}`]))},chapters:m.chapters.map(c=>({key:c.key,id:c.existing?.id||null,title:c.parsed.title,questions:c.questions.map(q=>({key:q.key,id:q.existing?.id||null,prompt:q.parsed.promptMarkdown}))}))}))}
 },{isolationLevel:'repeatable read',accessMode:'read only'})
}
export {publicPlan as getContentPreview}
export async function registerContentPreview(id:string,actor:string,origin:string,commit:string|null,configVersion:number|null){
 try{const files=await snapshot(id);await db.transaction(async tx=>{const current=await state(tx);await tx.insert(contentPreviews).values({id,actorId:actor,origin,commit,configVersion,manifest:files.manifest,baseline:current.baseline,bindings:emptyBindings(),expiresAt:new Date(Date.now()+3600000)})});return await publicPlan(id,actor)}catch(e){await discardPreview(id);throw e}
}
export async function createLocalContentPreview(directory:string,actor:string){
 const id=randomUUID(),target=previewDirectory(id)
 await mkdir(target,{recursive:true,mode:0o700})
 // Validate first, then copy only module MD and referenced images into an isolated snapshot.
 try{const checked=await checkContentDirectory(directory);const files=new Set<string>();for(const {module,images} of checked){files.add(module.sourcePath);for(const image of images)files.add(image.repositoryPath)}for(const file of files){const stat=await lstat(path.join(directory,file));if(!stat.isFile())fail('本地来源不能包含链接');await writeSnapshotFile(target,file,await readFile(path.join(directory,file)))}return await registerContentPreview(id,actor,'local',null,null)}catch(e){await discardPreview(id);throw e}
}
export async function updateContentBindings(id:string,actor:string,value:unknown){
 const bindings=validateBindings(value)
 await db.transaction(async tx=>{const p=await previewRecord(id,actor,tx,true);if(p.result)fail('已导入的预览不能修改');await tx.update(contentPreviews).set({bindings}).where(eq(contentPreviews.id,id))})
 return publicPlan(id,actor)
}
export async function applyContentPreview(id:string,actor:string){
 const before=await db.transaction(tx=>previewRecord(id,actor,tx));if(before.result)return before.result
 if(before.expiresAt.getTime()<Date.now())fail('预览已过期，请重新拉取')
 const files=await snapshot(id);if(files.manifest!==before.manifest)fail('预览内容已变化，请重新拉取')
 // Immutable files can be prepared before the DB transaction. Failure leaves unreferenced files only.
 for(const image of new Map([...files.images.values()].map(image=>[image.hash,image] as const)).values())await storeImage(image.bytes)
 const result=await db.transaction(async tx=>{
  await tx.execute(sql`select pg_advisory_xact_lock(72486211)`)
  const p=await previewRecord(id,actor,tx,true);if(p.result)return p.result
  if(p.expiresAt.getTime()<=Date.now())fail('预览已过期，请重新拉取')
  if(p.configVersion!==null){const [config]=await tx.select().from(contentRepository).where(eq(contentRepository.id,1));if(config?.version!==p.configVersion)fail('仓库配置已变化，请重新拉取')}
  const current=await state(tx);if(current.baseline!==p.baseline)fail('线上内容或配置已变化，请重新拉取预览')
  const report=plan(files.modules,p.origin,p.bindings,current);if(report.conflicts.length)fail(report.conflicts.join('\n'))
  const existingIds=report.modules.flatMap(m=>m.existing?[m.existing.id]:[])
  if(existingIds.length)await tx.delete(moduleSources).where(inArray(moduleSources.moduleId,existingIds))
  const moduleIds:string[]=[]
  for(const entry of report.modules){
   let moduleId=entry.existing?.id
   if(entry.changed){
    const categoryId=await ensureCategoryId(tx,current.categories,entry.parsed.categorySlug)
    const input:ModuleInput={title:entry.parsed.title,summary:entry.parsed.summary,descriptionMarkdown:entry.parsed.descriptionMarkdown,learningOutcomes:entry.parsed.learningOutcomes,difficulty:entry.parsed.difficulty,categoryId,lockVersion:entry.existing?.lockVersion,
     chapters:entry.chapters.map(c=>({chapterId:c.existing?.id,title:c.parsed.title,access:c.parsed.access,durationMinutes:c.parsed.durationMinutes,bodyMarkdown:c.parsed.bodyMarkdown,questions:c.questions.map(q=>({id:q.existing?.id,promptMarkdown:q.parsed.promptMarkdown,answer:q.parsed.answer,normalization:q.parsed.normalization,errorHint:q.parsed.errorHint}))}))}
    const saved=await saveModuleInTransaction(tx,moduleId||null,actor,input);moduleId=saved.module.id
    const storedChapters=await tx.select().from(chapters).where(eq(chapters.moduleId,moduleId))
    await tx.delete(chapterSources).where(eq(chapterSources.moduleId,moduleId))
    await tx.delete(moduleImages).where(eq(moduleImages.moduleId,moduleId))
    const assetMap=new Map<string,string>()
    const descriptionImages=new MarkdownIt({html:false}).parse(entry.parsed.descriptionMarkdown,{}).flatMap(t=>(t.children||[]).filter(c=>c.type==='image').map(c=>String(c.attrGet('src'))))
    for(const image of entry.parsed.images){
     if(assetMap.has(image.href))continue
     const file=files.images.get(bindingKey(entry.parsed.sourcePath,image.href))!
     let [asset]=await tx.select().from(assets).where(eq(assets.storageKey,file.hash))
     if(!asset)[asset]=await tx.insert(assets).values({storageKey:file.hash,originalName:path.basename(image.href),mediaType:file.mediaType,byteSize:file.bytes.length,sha256:Buffer.from(file.hash,'hex'),visibility:'private',uploadedBy:actor}).returning()
     assetMap.set(image.href,asset!.id)
     await tx.insert(moduleImages).values({moduleId,href:image.href,assetId:asset!.id,public:descriptionImages.includes(image.href)||image.href===entry.parsed.cover})
    }
    await tx.update(modules).set({coverAssetId:entry.parsed.cover ? assetMap.get(entry.parsed.cover)! : null}).where(eq(modules.id,moduleId))
    for(const c of entry.chapters){
     const stored=storedChapters.find(s=>s.position===c.parsed.position)!
     await tx.insert(chapterSources).values({chapterId:stored.id,moduleId,matchKey:c.parsed.matchKey})
     await tx.delete(questionSources).where(eq(questionSources.chapterId,stored.id))
     const questions=await tx.select().from(chapterQuestions).where(eq(chapterQuestions.chapterId,stored.id))
     for(const [index,q]of c.questions.entries())await tx.insert(questionSources).values({questionId:questions.find(s=>s.position===index+1)!.id,chapterId:stored.id,matchKey:q.parsed.matchKey})
     await tx.delete(chapterAssets).where(eq(chapterAssets.chapterId,stored.id))
     // It is safe to associate all source images with a chapter only if actually referenced there.
     const sourceText=[c.parsed.bodyMarkdown,...c.parsed.questions.map(q=>q.promptMarkdown)].join('\n')
     const refs=new Set(new MarkdownIt({html:false}).parse(sourceText,{}).flatMap(t=>(t.children||[]).filter(t=>t.type==='image').map(t=>String(t.attrGet('src')))))
     for(const assetId of new Set([...refs].map(h=>assetMap.get(h)).filter((id):id is string=>!!id)))await tx.insert(chapterAssets).values({chapterId:stored.id,assetId})
    }
   }
   moduleIds.push(moduleId!)
   await tx.insert(moduleSources).values({moduleId:moduleId!,origin:p.origin,sourcePath:entry.parsed.sourcePath,sourceHash:entry.parsed.sourceHash,commit:p.commit}).onConflictDoUpdate({target:moduleSources.moduleId,set:{origin:p.origin,sourcePath:entry.parsed.sourcePath,sourceHash:entry.parsed.sourceHash,commit:p.commit,syncedAt:new Date()}})
  }
  await pruneUnusedCategories(tx)
  const result={moduleIds};await tx.update(contentPreviews).set({result}).where(eq(contentPreviews.id,id));return result
 }).catch(error=>{
  if(error?.statusCode)throw error
  const code=typeof error?.code==='string'&&/^[0-9A-Z]{5}$/.test(error.code)?error.code:'unknown'
  console.error(`[content-sync] transaction failed (${code})`)
  throw createError({statusCode:500,message:'导入事务失败，线上内容未更改'})
 })
 await discardPreview(id).catch(()=>{})
 return result
}

export async function contentPreviewImage(id:string,actor:string,source:string,href:string){
 const p=await db.transaction(tx=>previewRecord(id,actor,tx))
 if(p.result||p.expiresAt.getTime()<Date.now())fail('图片预览已过期',404)
 const files=await snapshot(id)
 if(files.manifest!==p.manifest)fail('预览内容已变化')
 const image=files.images.get(bindingKey(source,href))
 if(!image)fail('图片不存在',404)
 return image
}

export async function deleteContentPreview(id:string,actor:string){
 await db.transaction(async tx=>{await tx.execute(sql`select pg_advisory_xact_lock(72486211)`);await previewRecord(id,actor,tx,true);await tx.delete(contentPreviews).where(eq(contentPreviews.id,id))})
 await discardPreview(id)
 return {deleted:true}
}
