import {randomUUID} from 'node:crypto'
import {and,eq,sql} from 'drizzle-orm'
import {createError} from 'h3'
import {db} from '../db'
import {contentRepository,contentPreviews} from '../db/schema'
import {encryptContentToken,decryptContentToken} from '../content/credentials'
import {parseRepositoryConfig,githubCommit,githubDirectory,fetchGithubSnapshot} from '../content/github'
import {previewDirectory,discardPreview} from '../content/storage'
import {registerContentPreview} from './content-sync'
export async function contentRepositorySettings(actor:string){
 const [row]=await db.select().from(contentRepository).where(eq(contentRepository.id,1))
 const [latest]=await db.select({id:contentPreviews.id,commit:contentPreviews.commit,createdAt:contentPreviews.createdAt,result:contentPreviews.result}).from(contentPreviews).where(and(eq(contentPreviews.actorId,actor),sql`${contentPreviews.configVersion} is not null`)).orderBy(sql`${contentPreviews.createdAt} desc`).limit(1)
 return {repository:row?.repository||'',branch:row?.branch||'main',directory:row?.directory||'',hasToken:!!row?.tokenCipher,version:row?.version||0,latest:latest||null}
}
export async function saveContentRepository(body:Record<string,unknown>){
 const config=parseRepositoryConfig(body)
 if(body.token!==undefined&&(typeof body.token!=='string'||body.token.length>4096||/[\r\n]/.test(body.token)))throw createError({statusCode:400,message:'Token 格式无效'})
 return db.transaction(async tx=>{
  await tx.execute(sql`select pg_advisory_xact_lock(72486211)`)
  const [old]=await tx.select().from(contentRepository).where(eq(contentRepository.id,1))
  if(body.version!==(old?.version||0))throw createError({statusCode:409,message:'仓库配置已变化，请重新读取'})
  const token=typeof body.token==='string'?body.token.trim():''
  const tokenCipher=body.clearToken===true?'':token?encryptContentToken(token):old?.tokenCipher||''
  await tx.insert(contentRepository).values({id:1,...config,tokenCipher}).onConflictDoUpdate({target:contentRepository.id,set:{...config,tokenCipher,version:(old?.version||0)+1,updatedAt:new Date()}})
  return {saved:true}
 })
}
async function credentials(){const [row]=await db.select().from(contentRepository).where(eq(contentRepository.id,1));if(!row?.tokenCipher)throw createError({statusCode:400,message:'请先保存仓库和只读 Token'});return {config:row,token:decryptContentToken(row.tokenCipher)}}
export async function testContentRepository(){const {config,token}=await credentials();const commit=await githubCommit(config,token);await githubDirectory(config,token,commit);return {commit,connected:true}}
export async function pullContentRepository(actor:string){
 const {config,token}=await credentials(),id=randomUUID()
 try{const commit=await fetchGithubSnapshot(config,token,previewDirectory(id));return await registerContentPreview(id,actor,`github:${config.repository.toLowerCase()}:${config.directory}`,commit,config.version)}catch(e){await discardPreview(id);throw e}
}
