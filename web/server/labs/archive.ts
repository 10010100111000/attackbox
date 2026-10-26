import {fromBuffer, type Entry} from 'yauzl'
import {createHash} from 'node:crypto'
import {parseDocument} from 'yaml'
export const MAX_PACKAGE=8*1024*1024,MAX_EXPANDED=32*1024*1024
export function packagePath(path:string){if(!/^[A-Za-z0-9_.\-/]+$/.test(path)||path.startsWith('/')||path.split('/').some(p=>!p||p==='.'||p==='..')||path.length>240)throw Error('部署包包含无效路径');return path}
export const sha=(value:Buffer|string)=>createHash('sha256').update(value).digest('hex')
export function manifestHash(manifest:Record<string,string>){return sha(Object.keys(manifest).sort().map(k=>`${k}\0${manifest[k]}\n`).join(''))}
export async function unpack(data:Buffer,filename:string):Promise<Map<string,Buffer>>{
 if(!data.length||data.length>MAX_PACKAGE)throw Error('部署包大小需在 8 MiB 以内')
 if(/\.ya?ml$/i.test(filename))return new Map([['compose.yml',data]])
 if(!/\.zip$/i.test(filename))throw Error('请上传 ZIP 或 Compose YAML 文件')
 return new Promise((resolve,reject)=>fromBuffer(data,{lazyEntries:true,decodeStrings:true,validateEntrySizes:true},(err,zip)=>{
  if(err||!zip){reject(Error('ZIP 文件无法读取'));return}
  const files=new Map<string,Buffer>(),names=new Set<string>();let count=0,total=0,done=false
  const fail=(e:unknown)=>{if(done)return;done=true;zip.close();reject(e instanceof Error?e:Error('部署包无效'))}
  zip.on('error',()=>fail(Error('ZIP 文件无效')))
  zip.on('entry',(e:Entry)=>{try{
   if(++count>512)throw Error('部署包最多包含 512 项')
   const directory=e.fileName.endsWith('/'),path=packagePath(directory?e.fileName.slice(0,-1):e.fileName),mode=(e.externalFileAttributes>>>16)&0xf000
   if(names.has(path)||mode&&mode!==(directory?0x4000:0x8000)||e.generalPurposeBitFlag&1)throw Error('部署包不能包含重复路径、链接、特殊文件或加密条目')
   names.add(path);if(directory){zip.readEntry();return}
   if(e.uncompressedSize>4*1024*1024||total+e.uncompressedSize>MAX_EXPANDED)throw Error('文件或解压后总大小超出限制')
   zip.openReadStream(e,(err,stream)=>{if(err||!stream){fail(Error('ZIP 条目无法读取'));return}const chunks:Buffer[]=[];let bytes=0
    stream.on('error',()=>fail(Error('ZIP 条目无效')));stream.on('data',(chunk:Buffer)=>{bytes+=chunk.length;total+=chunk.length;if(bytes>4*1024*1024||total>MAX_EXPANDED){stream.destroy();fail(Error('解压大小超出限制'))}else chunks.push(chunk)})
    stream.on('end',()=>{if(done)return;files.set(path,Buffer.concat(chunks));zip.readEntry()})
   })
  }catch(e){fail(e)}})
  zip.on('end',()=>{if(!done){done=true;resolve(files)}});zip.readEntry()
 }))
}
export function previewFiles(files:Map<string,Buffer>,selected?:string){
 const composeFiles=[...files.keys()].filter(k=>/(^|\/)(compose|docker-compose)\.ya?ml$/i.test(k)).sort()
 const file=selected||composeFiles[0];if(!file||!files.has(file))throw Error('部署包中未找到 Compose 文件')
 const content=files.get(file)!.toString('utf8');if(Buffer.byteLength(content)>1024*1024)throw Error('Compose 文件不能超过 1 MiB')
 const doc=parseDocument(content,{uniqueKeys:true});if(doc.errors.length)throw Error('Compose YAML 格式无效')
 let raw:any;try{raw=doc.toJS({maxAliasCount:0})}catch{throw Error('暂不支持 YAML aliases')}
 if(!raw||typeof raw!=='object'||Array.isArray(raw)||!raw.services||typeof raw.services!=='object')throw Error('Compose 需要 services 定义')
 const issues:string[]=[],notes:string[]=[],missingFiles:string[]=[],variables=new Set<string>()
 for(const m of content.matchAll(/\$\{([A-Za-z_][A-Za-z0-9_]*)[^}]*\}/g))variables.add(m[1]!)
 const services=Object.entries(raw.services).map(([name,value])=>{
  const s=value as any;if(!s||typeof s!=='object')throw Error('服务配置无效')
  for(const key of ['extends','env_file','profiles','privileged','devices','network_mode'])if(key in s)issues.push(`${name}：暂不支持 ${key}`)
  const resolve=(base:string,ref:unknown):string|null=>{
   if(typeof ref!=='string'||ref.startsWith('/')||/[\\:$]/.test(ref)){issues.push(`${name}：仅支持包内相对文件路径，路径不能使用变量`);return null}
   const stack=base?base.split('/'):[]
   for(const part of ref.split('/')){if(!part||part==='.')continue;if(part==='..'){if(!stack.length){issues.push(`${name}：文件引用超出部署包`);return null}stack.pop()}else stack.push(part)}
   const path=stack.join('/');try{if(path)packagePath(path)}catch{issues.push(`${name}：文件路径无效`);return null}return path
  }
  const directory=file.split('/').slice(0,-1).join('/'),hasDirectory=(path:string)=>[...files.keys()].some(k=>!path||k.startsWith(path+'/'))
  let build:{context:string;dockerfile:string;target?:string}|undefined
  if(s.build!==undefined){
   const b=typeof s.build==='string'?{context:s.build}:s.build
   if(!b||typeof b!=='object'||Array.isArray(b))issues.push(`${name}：构建配置无效`)
   else{
    for(const key of Object.keys(b))if(!['context','dockerfile','args','target'].includes(key))issues.push(`${name}：暂不支持 build.${key}`)
    const context=resolve(directory,b.context??'.')
    if(context!==null){if(!hasDirectory(context))missingFiles.push(context||'.');const dockerfile=resolve(context,b.dockerfile??'Dockerfile');if(dockerfile!==null){if(!files.has(dockerfile))missingFiles.push(dockerfile);build={context:context||'.',dockerfile,...(typeof b.target==='string'?{target:b.target}:{})}}}
    notes.push(`${name}：管理员准备时构建，学员启动不会构建或拉取镜像`)
   }
  }
  const ports=(Array.isArray(s.ports)?s.ports:[]).flatMap((p:any)=>{const target=typeof p==='object'?p.target:String(p).split(':').at(-1)?.split('/')[0],port=Number(target),protocol: 'tcp'|'udp'=typeof p==='object'?p.protocol==='udp'?'udp':'tcp':String(p).endsWith('/udp')?'udp':'tcp';return Number.isInteger(port)&&port>0&&port<=65535?[{port,protocol}]:[]})
  if(s.ports?.length)notes.push(`${name}：宿主端口映射将转换为私有服务访问`)
  if(!s.image&&s.build===undefined)issues.push(`${name}：需要 image 或包内 build`)
  if(s.volumes!==undefined&&!Array.isArray(s.volumes))throw Error('服务 volumes 需要数组')
  for(const mount of s.volumes||[]){let source:string|undefined,ro=false,isBind=false
   if(typeof mount==='string'){const parts=mount.split(':');source=parts.length>1?parts[0]:undefined;ro=parts[2]?.split(',').includes('ro')||false;isBind=!!source&&(source.startsWith('.')||source.startsWith('/')||source.includes('/'))}
   else if(mount?.type==='bind'){source=mount.source;ro=mount.read_only===true;isBind=true}
   if(isBind){const path=resolve(directory,source);if(path===null)continue;if(!files.has(path)&&!hasDirectory(path))missingFiles.push(path||'.');if(!ro)notes.push(`${name}：可写挂载使用实例独立副本，释放时删除`)}
  }
  const networks=s.networks?Array.isArray(s.networks)?s.networks:Object.keys(s.networks):['default']
  return {name,image:typeof s.image==='string'?s.image:'',networks,ports,...(build?{build}:{})}
 })
 if(services.length<1||services.length>8)issues.push('默认预算最多支持 8 个服务')
 for(const key of ['include','configs','secrets'])if(key in raw)issues.push(`暂不支持 ${key}`)
 return {files:files.size,composeFiles,services,networks:[...new Set(services.flatMap(s=>s.networks))] as string[],variables:[...variables].sort(),missingFiles:[...new Set(missingFiles)],issues,notes}
}
