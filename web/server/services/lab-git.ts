import {randomUUID} from 'node:crypto'
import {createError} from 'h3'
import {db} from '../db'
import {labBundles} from '../db/schema'
import {githubReader,parseLabRepository,parseLabToken,resolveLabCommit,readLabSnapshot} from '../labs/github'
import {sealVariables,openVariables} from '../labs/credentials'
import {previewFiles} from '../labs/archive'
import {saveBundleFiles} from '../labs/storage'
import {bundleRecord,previewLabBundle} from './compose-templates'
const activeImports=new Set<string>()
// Bound source credentials use distinct AAD and never borrow tutorial credentials.
export async function labGitAction(actor:string,body:any,bundleId?:string){
 if(!body||typeof body!=='object'||Array.isArray(body))throw createError({statusCode:400,message:'导入参数无效'})
 if(activeImports.has(actor)||activeImports.size>=2)throw createError({statusCode:409,message:'有导入任务正在执行，请稍后手动重试'})
 activeImports.add(actor)
 try{
  if(bundleId&&!['check','import'].includes(body.action))throw Error('操作无效')
  const old=bundleId?await bundleRecord(bundleId):null
  if(old&&!old.source)throw Error('此部署包不是 Git 来源')
  const config=old?.source||parseLabRepository(body),token=old?openVariables('git:'+old.id,old.gitTokenCipher).token||'':parseLabToken(body.token)
  const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),120000)
  try{
   const read=githubReader(config,token,controller.signal),commit=await resolveLabCommit(config,read)
   if(old&&body.action==='check')return {currentCommit:old.source!.commit,latestCommit:commit,changed:commit!==old.source!.commit}
   if(old&&body.action!=='import')throw Error('操作无效')
   if(old&&(typeof body.expectedCommit!=='string'||body.expectedCommit!==commit))throw createError({statusCode:409,message:'仓库提交已变化，请重新检查更新后确认'})
   if(old&&commit===old.source!.commit)return {bundle:await previewLabBundle(old.id,typeof body.file==='string'?body.file:undefined),unchanged:true}
   const files=await readLabSnapshot(config,commit,read),available=previewFiles(files)
   const file=available.composeFiles.includes(body.file)?body.file:undefined;previewFiles(files,file)
   const id=randomUUID(),stored=await saveBundleFiles(id,files),source={repository:config.repository,branch:config.branch,directory:config.directory,commit}
   const gitTokenCipher=token?sealVariables('git:'+id,{token}):null
   try{await db.insert(labBundles).values({id,actorId:actor,...stored,source,gitTokenCipher})}catch{throw createError({statusCode:500,message:'部署包记录保存失败，请联系管理员'})}
   return {bundle:await previewLabBundle(id,file),unchanged:old?.hash===stored.hash}
  }finally{clearTimeout(timer);controller.abort()}
 }catch(e){if((e as any).statusCode)throw e;throw createError({statusCode:400,message:e instanceof Error?e.message:'Git 导入失败'})}finally{activeImports.delete(actor)}
}
