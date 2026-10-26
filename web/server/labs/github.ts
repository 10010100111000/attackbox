import {packagePath,MAX_EXPANDED} from './archive.ts'
export interface LabRepository {repository:string;branch:string;directory:string}
export function parseLabRepository(body:Record<string,unknown>):LabRepository {
 if(typeof body.repository!=='string'||body.branch!==undefined&&typeof body.branch!=='string'||body.directory!==undefined&&typeof body.directory!=='string')throw Error('请填写有效的 GitHub 仓库、分支和目录')
 const repository=body.repository.trim().replace(/^https:\/\/github\.com\//,'').replace(/\/$/,'').replace(/\.git$/,'')
 if(repository.length>255||!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository)||repository.split('/').some(p=>p==='.'||p==='..'))throw Error('首版支持 GitHub owner/repository 或 HTTPS 仓库链接')
 const branch=(body.branch||'main').trim(),directory=(body.directory||'').trim().replace(/^\.\/$/,'').replace(/\/$/,'')
 if(!branch||branch.length>200||/[\s\\?#]/.test(branch))throw Error('分支或标签无效')
 if(directory){packagePath(directory);if(directory.split('/').length>16)throw Error('目录层级过深')}
 return {repository,branch,directory}
}
export function parseLabToken(value:unknown){if(value===undefined)return '';if(typeof value!=='string'||value.length>4096||/[\r\n]/.test(value))throw Error('Token 格式无效');return value.trim()}
// Fixed host, no redirects, bounded response and lifetime; no checkout, hooks or scripts.
export function githubReader(config:LabRepository,token:string,signal:AbortSignal){
 return async(suffix:string)=>{
  let response:Response
  try{response=await fetch(`https://api.github.com/repos/${config.repository}/${suffix}`,{headers:{Accept:'application/vnd.github+json','X-GitHub-Api-Version':'2022-11-28',...(token?{Authorization:`Bearer ${token}`}:{})},redirect:'error',signal})}catch{throw Error('GitHub 访问失败或导入超时，请手动重试')}
  if(!response.ok){await response.body?.cancel();throw Error([401,403,404].includes(response.status)?'仓库、分支或目录不可读，请检查地址及只读 Token 权限':'GitHub 访问失败，请手动重试')}
  const reader=response.body?.getReader();if(!reader)throw Error('GitHub 响应为空')
  const chunks:Uint8Array[]=[];let size=0
  try{for(;;){const {done,value}=await reader.read();if(done)break;size+=value.length;if(size>6*1024*1024)throw Error('GitHub 响应超出限制');chunks.push(value)}}catch(e){await reader.cancel().catch(()=>{});throw e}
  try{return JSON.parse(Buffer.concat(chunks).toString('utf8'))}catch{throw Error('GitHub 响应无效')}
 }
}
export type GitReader=(suffix:string)=>Promise<any>
export async function resolveLabCommit(config:LabRepository,read:GitReader){const r=await read(`commits/${encodeURIComponent(config.branch)}`);if(!/^[a-f0-9]{40}$/.test(r.sha))throw Error('GitHub 提交标识无效');return r.sha as string}
export async function readLabSnapshot(config:LabRepository,commit:string,read:GitReader){
 if(!/^[a-f0-9]{40}$/.test(commit))throw Error('提交标识无效')
 let tree=(await read(`git/commits/${commit}`)).tree?.sha
 const validTree=()=>{if(typeof tree!=='string'||!/^[a-f0-9]{40}$/.test(tree))throw Error('GitHub 目录标识无效')};validTree()
 for(const component of config.directory.split('/').filter(Boolean)){
  const listing=await read(`git/trees/${tree}`)
  if(listing.truncated||!Array.isArray(listing.tree))throw Error('目录过大或无效，请选择更具体的目录')
  tree=listing.tree.find((e:any)=>e.path===component&&e.type==='tree')?.sha;validTree()
 }
 const listing=await read(`git/trees/${tree}?recursive=1`)
 if(listing.truncated||!Array.isArray(listing.tree)||listing.tree.length>2000)throw Error('目录过大，请选择更具体的靶场目录')
 const entries=listing.tree.filter((e:any)=>e.type!=='tree'),seen=new Set<string>();let total=0
 if(!entries.length||entries.length>512)throw Error('部署包需要 1–512 个文件')
 for(const e of entries){packagePath(e.path);if(seen.has(e.path)||e.type!=='blob'||!['100644','100755'].includes(e.mode)||!/^[a-f0-9]{40}$/.test(e.sha))throw Error('目录不支持链接、子模块或重复路径');seen.add(e.path);if(!Number.isSafeInteger(e.size)||e.size<0||e.size>4*1024*1024)throw Error('单文件不能超过 4 MiB');total+=e.size;if(total>MAX_EXPANDED)throw Error('目录总大小不能超过 32 MiB')}
 const files=new Map<string,Buffer>();let next=0
 await Promise.all(Array.from({length:Math.min(4,entries.length)},async()=>{
  for(;;){const e=entries[next++];if(!e)return;const blob=await read(`git/blobs/${e.sha}`);if(blob.encoding!=='base64'||typeof blob.content!=='string'||blob.content.length>6*1024*1024)throw Error('GitHub 文件编码无效');const data=Buffer.from(blob.content,'base64');if(data.length!==e.size)throw Error('GitHub 文件大小不一致');files.set(e.path,data)}
 }))
 return files
}
