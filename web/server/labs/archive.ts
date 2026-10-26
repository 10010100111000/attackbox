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
  for(const key of ['build','extends','env_file','profiles','privileged','devices','network_mode'])if(key in s)issues.push(`${name}：暂不支持 ${key}`)
  const ports=(Array.isArray(s.ports)?s.ports:[]).flatMap((p:any)=>{const target=typeof p==='object'?p.target:String(p).split(':').at(-1)?.split('/')[0],port=Number(target),protocol: 'tcp'|'udp'=typeof p==='object'?p.protocol==='udp'?'udp':'tcp':String(p).endsWith('/udp')?'udp':'tcp';return Number.isInteger(port)&&port>0&&port<=65535?[{port,protocol}]:[]})
  if(s.ports?.length)notes.push(`${name}：宿主端口映射将转换为私有服务访问`)
  if(!s.image)issues.push(`${name}：缺少 image`)
  for(const mount of s.volumes||[]){let source:string|undefined,ro=false
   if(typeof mount==='string'){const parts=mount.split(':');source=parts.length>1?parts[0]:undefined;ro=parts[2]?.split(',').includes('ro')||false}
   else if(mount?.type==='bind'){source=mount.source;ro=mount.read_only===true}
   if(source&&(source.startsWith('.')||source.startsWith('/'))){if(!ro)issues.push(`${name}：包内文件需只读挂载`);if(source.startsWith('/')){issues.push(`${name}：不支持宿主机路径`);continue}
    const stack=file.split('/').slice(0,-1);for(const part of source.split('/')){if(!part||part==='.')continue;if(part==='..'){if(!stack.length){issues.push(`${name}：文件引用超出部署包`);break}stack.pop()}else stack.push(part)}
    const path=stack.join('/');if(!files.has(path))missingFiles.push(path)
   }
  }
  const networks=s.networks?Array.isArray(s.networks)?s.networks:Object.keys(s.networks):['default']
  return {name,image:typeof s.image==='string'?s.image:'',networks,ports}
 })
 if(services.length<1||services.length>8)issues.push('默认预算最多支持 8 个服务')
 for(const key of ['include','configs','secrets'])if(key in raw)issues.push(`暂不支持 ${key}`)
 return {files:files.size,composeFiles,services,networks:[...new Set(services.flatMap(s=>s.networks))] as string[],variables:[...variables].sort(),missingFiles:[...new Set(missingFiles)],issues,notes}
}
