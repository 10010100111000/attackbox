import {randomUUID} from 'node:crypto'
import {and,eq,sql} from 'drizzle-orm'
import {createError} from 'h3'
import {db} from '../db'
import {labBundles,labPreparations,targetTemplates} from '../db/schema'
import {previewFiles,unpack} from '../labs/archive'
import {readBundleFiles,saveBundleFiles} from '../labs/storage'
import {openVariables,sealVariables} from '../labs/credentials'
import type {ComposeSettings} from '../../shared/compose-labs'
export const uuid=(v:unknown)=>{if(typeof v!=='string'||!/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(v))throw createError({statusCode:400,message:'标识无效'});return v}
export async function bundleRecord(id:string){const [b]=await db.select().from(labBundles).where(eq(labBundles.id,uuid(id)));if(!b)throw createError({statusCode:404,message:'部署包不存在'});return b}
export async function uploadLabBundle(actor:string,data:Buffer,filename:string){
 try{const files=await unpack(data,filename),preview=previewFiles(files),id=randomUUID(),stored=await saveBundleFiles(id,files);await db.insert(labBundles).values({id,actorId:actor,...stored});return {id,hash:stored.hash,...preview}}
 catch(e){if((e as any).statusCode)throw e;throw createError({statusCode:400,message:e instanceof Error?e.message:'部署包无法读取'})}
}
export async function previewLabBundle(id:string,file?:string){const b=await bundleRecord(id);try{return {id:b.id,hash:b.hash,source:b.source?{...b.source,hasToken:!!b.gitTokenCipher}:null,...previewFiles(await readBundleFiles(b.id,b.manifest),file)}}catch{throw createError({statusCode:400,message:'部署包校验或 Compose 解析失败'})}}
export function publicTemplate<T extends {variablesCipher:string|null}>(t:T){const {variablesCipher,...safe}=t;return safe}
export async function composeTemplate(id:string){const [t]=await db.select().from(targetTemplates).where(eq(targetTemplates.id,uuid(id)));if(!t||t.runtime!=='compose')throw createError({statusCode:404,message:'Compose 模板不存在'});return t}
export async function saveComposeTemplate(actor:string,id:string|null,body:any){
 if(!body||typeof body!=='object'||Array.isArray(body))throw createError({statusCode:400,message:'表单无效'})
 const name=typeof body.name==='string'?body.name.trim():'',description=typeof body.description==='string'?body.description.trim():'',accessInstructions=typeof body.accessInstructions==='string'?body.accessInstructions.trim():''
 if(!name||name.length>120||description.length>500||accessInstructions.length>4000||typeof body.enabled!=='boolean')throw createError({statusCode:400,message:'请检查名称、说明和启用状态'})
 const bundle=await bundleRecord(body.bundleId),preview=await previewLabBundle(bundle.id,body.settings?.file)
 if(preview.issues.length||preview.missingFiles.length)throw createError({statusCode:400,message:'请先解决部署包中的问题和缺失文件'})
 const s=body.settings as ComposeSettings
 if(!s||!preview.composeFiles.includes(s.file)||!preview.services.some(x=>x.name===s.entryService&&x.networks.includes(s.entryNetwork))||!Number.isInteger(s.entryPort)||s.entryPort<1||s.entryPort>65535||![24,25,26,27,28].includes(s.prefixBits))throw createError({statusCode:400,message:'请检查入口服务、网络、端口和网络大小'})
 const settings:ComposeSettings={file:s.file,entryService:s.entryService,entryNetwork:s.entryNetwork,entryPort:s.entryPort,prefixBits:s.prefixBits}
 const templateId=id?uuid(id):randomUUID()
 return db.transaction(async tx=>{
  await tx.execute(sql`select pg_advisory_xact_lock(72486212)`)
  const [old]=id?await tx.select().from(targetTemplates).where(eq(targetTemplates.id,templateId)):[]
  if(id&&(!old||old.runtime!=='compose'))throw createError({statusCode:404,message:'Compose 模板不存在'})
  if(old&&old.lockVersion!==body.lockVersion)throw createError({statusCode:409,message:'模板已被修改，当前输入已保留，请重新读取'})
  const previous=openVariables(templateId,old?.variablesCipher||null),variables:Record<string,string>=Object.create(null)
  const clear=body.clearVariables||[]
  if(!Array.isArray(clear)||clear.length>64||clear.some(k=>typeof k!=='string'||!preview.variables.includes(k)))throw createError({statusCode:400,message:'变量清除列表无效'})
  for(const key of preview.variables)if(Object.hasOwn(previous,key)&&!clear.includes(key))variables[key]=previous[key]!
  if(body.variables!==undefined){if(!body.variables||typeof body.variables!=='object'||Array.isArray(body.variables)||Object.keys(body.variables).length>64)throw createError({statusCode:400,message:'部署变量无效'});for(const [key,value]of Object.entries(body.variables)){if(!preview.variables.includes(key)||typeof value!=='string'||value.length>8192)throw createError({statusCode:400,message:'部署变量无效'});if(value!=='')variables[key]=value}}
  const ordered=(v:Record<string,string>)=>JSON.stringify(Object.fromEntries(Object.entries(v).sort(([a],[b])=>a.localeCompare(b))))
  if(Buffer.byteLength(ordered(variables))>32768)throw createError({statusCode:400,message:'部署变量总长度超出限制'})
  const oldBundle=old?.bundleId?await tx.select({hash:labBundles.hash}).from(labBundles).where(eq(labBundles.id,old.bundleId)):[]
  const changed=!old||oldBundle[0]?.hash!==bundle.hash||Object.entries(settings).some(([key,value])=>old.composeSettings?.[key as keyof ComposeSettings]!==value)||ordered(previous)!==ordered(variables)
  if(changed&&old){const [active]=await tx.select({id:labPreparations.id}).from(labPreparations).where(and(eq(labPreparations.templateId,templateId),eq(labPreparations.templateVersion,old.composeRevision),sql`${labPreparations.state} in ('preparing','cancelling','unknown')`));if(active)throw createError({statusCode:409,message:'请先查询或取消当前准备任务，再修改部署配置'})}
  const revision=(old?.composeRevision||0)+(changed?1:0)
  if(body.enabled){const [ready]=await tx.select({id:labPreparations.id}).from(labPreparations).where(and(eq(labPreparations.templateId,templateId),eq(labPreparations.templateVersion,revision),sql`${labPreparations.deploymentId} is not null`));if(!ready)throw createError({statusCode:409,message:'请先保存未启用模板，并在节点准备成功后启用'})}
  const values={name,description,accessInstructions,runtime:'compose',image:'',ports:[{port:s.entryPort,protocol:'tcp' as const}],sourceUrl:'',bundleId:bundle.id,composeSettings:settings,composeRevision:revision,variablesCipher:changed?sealVariables(templateId,variables):old!.variablesCipher,variableKeys:Object.keys(variables).sort(),enabled:body.enabled,lockVersion:(old?.lockVersion||0)+1,updatedAt:new Date()}
  const [saved]=old?await tx.update(targetTemplates).set(values).where(eq(targetTemplates.id,templateId)).returning():await tx.insert(targetTemplates).values({id:templateId,...values}).returning()
  return publicTemplate(saved!)
 })
}
