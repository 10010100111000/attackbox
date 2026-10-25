import {createError} from 'h3'
import {mkdir} from 'node:fs/promises'
import {writeSnapshotFile} from './storage'
export interface RepositoryConfig {repository:string;branch:string;directory:string}
export function parseRepositoryConfig(body:Record<string,unknown>):RepositoryConfig{
 if(typeof body.repository!=='string'||body.branch!==undefined&&typeof body.branch!=='string'||body.directory!==undefined&&typeof body.directory!=='string')throw createError({statusCode:400,message:'仓库、分支和目录必须是文本'})
 const repository=String(body.repository||'').trim().replace(/^https:\/\/github\.com\//,'').replace(/\.git\/?$/,'').replace(/\/$/,'')
 if(!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository)||repository.split('/').some(p=>p==='.'||p==='..'))throw createError({statusCode:400,message:'请填写 GitHub owner/repository 或标准 HTTPS 仓库链接'})
 const branch=String(body.branch||'main').trim(),directory=String(body.directory||'').trim().replace(/^\.\/$/,'').replace(/\/$/,'')
 if(!branch||branch.length>200||/[\s\\?#]/.test(branch))throw createError({statusCode:400,message:'分支名无效'})
 if(directory.length>500||directory.split('/').length>16)throw createError({statusCode:400,message:'教程目录过长或层级过深'})
 if(directory&&(directory.startsWith('/')||directory.includes('\\')||directory.split('/').some(p=>!p||p==='.'||p==='..')))throw createError({statusCode:400,message:'教程目录必须是仓库内相对路径'})
 return {repository,branch,directory}
}
// Fixed GitHub API host; no arbitrary URLs, redirects, shell commands or repo scripts.
export async function githubJson(config:RepositoryConfig,token:string,suffix:string,signal?:AbortSignal):Promise<any>{
 try{return await $fetch(`https://api.github.com/repos/${config.repository}/${suffix}`,{headers:{Authorization:`Bearer ${token}`,Accept:'application/vnd.github+json','X-GitHub-Api-Version':'2022-11-28'},timeout:20000,retry:0,redirect:'error',signal})}
 catch(e:any){const status=e?.response?.status;throw createError({statusCode:502,message:status===401||status===403?'GitHub 凭据无效、权限不足或访问受限':status===404?'仓库、分支或目录不存在，或 Token 没有读取权限':'GitHub 访问失败，请稍后手动重试'})}
}
export async function githubCommit(config:RepositoryConfig,token:string,signal?:AbortSignal){const result=await githubJson(config,token,`commits/${encodeURIComponent(config.branch)}`,signal);if(!/^[a-f0-9]{40}$/.test(result.sha))throw createError({statusCode:502,message:'GitHub 返回无效提交'});return result.sha as string}
export async function githubDirectory(config:RepositoryConfig,token:string,commit:string,signal?:AbortSignal){
 let tree=(await githubJson(config,token,`git/commits/${commit}`,signal)).tree?.sha
 for(const component of config.directory.split('/').filter(Boolean)){
  const result=await githubJson(config,token,`git/trees/${tree}`,signal)
  const entry=result.tree?.find((e:any)=>e.path===component&&e.type==='tree')
  if(!entry)throw createError({statusCode:400,message:'指定教程目录不存在'})
  tree=entry.sha
 }
 return tree as string
}
export async function fetchGithubSnapshot(config:RepositoryConfig,token:string,directory:string){
 await mkdir(directory,{recursive:true,mode:0o700})
 const signal=AbortSignal.timeout(120000)
 const commit=await githubCommit(config,token,signal),tree=await githubDirectory(config,token,commit,signal)
 const result=await githubJson(config,token,`git/trees/${tree}?recursive=1`,signal)
 if(result.truncated||!Array.isArray(result.tree)||result.tree.length>2000)throw createError({statusCode:400,message:'内容目录过大，请选择更具体的教程目录'})
 let size=0
 const files=result.tree.filter((e:any)=>e.type!=='tree')
 for(const file of files){
  if(file.mode==='120000'||file.type!=='blob')throw createError({statusCode:400,message:'教程目录不支持符号链接或子模块'})
  if(!(/\.md$/.test(file.path)||/\.(?:png|jpe?g|webp|gif|avif)$/i.test(file.path)))continue
  size+=file.size||0
  if(file.size>10*1024*1024||size>64*1024*1024)throw createError({statusCode:400,message:'教程目录文件超出大小限制'})
  const blob=await githubJson(config,token,`git/blobs/${file.sha}`,signal)
  if(blob.encoding!=='base64'||typeof blob.content!=='string')throw createError({statusCode:502,message:'GitHub 文件编码无效'})
  const data=Buffer.from(blob.content,'base64')
  if(data.length!==file.size||data.length>10*1024*1024)throw createError({statusCode:502,message:'GitHub 文件大小不一致'})
  await writeSnapshotFile(directory,file.path,data)
 }
 return commit
}
