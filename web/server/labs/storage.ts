import {mkdir,writeFile,readFile,rename,rm,lstat} from 'node:fs/promises'
import {resolve,join} from 'node:path'
import {randomUUID} from 'node:crypto'
import {packagePath,sha,manifestHash} from './archive'
export const bundleRoot=()=>resolve(process.env.LAB_DATA_DIR||'/var/lib/attackbox-web/labs')
export async function saveBundleFiles(id:string,files:Map<string,Buffer>){
 const root=bundleRoot();await mkdir(root,{recursive:true,mode:0o700});const info=await lstat(root);if(!info.isDirectory()||info.isSymbolicLink()||(info.mode&0o077)!==0)throw Error('部署包持久目录必须为私有目录（0700）');const tmp=join(root,'.upload-'+randomUUID());await mkdir(tmp,{mode:0o700});const manifest:Record<string,string>={}
 try{for(const [name,data]of files){packagePath(name);const path=join(tmp,...name.split('/'));await mkdir(resolve(path,'..'),{recursive:true,mode:0o700});await writeFile(path,data,{mode:0o600,flag:'wx'});manifest[name]=sha(data)}await rename(tmp,join(root,id));return {manifest,hash:manifestHash(manifest)}}catch(e){await rm(tmp,{recursive:true,force:true});throw e}
}
export async function readBundleFiles(id:string,manifest:Record<string,string>){
 if(!/^[a-f0-9-]{36}$/.test(id))throw Error('部署包标识无效')
 const files=new Map<string,Buffer>();let total=0
 for(const [name,hash]of Object.entries(manifest)){packagePath(name);const path=join(bundleRoot(),id,...name.split('/'));for(let p=path;p!==bundleRoot();p=resolve(p,'..'))if((await lstat(p)).isSymbolicLink())throw Error('部署包文件已变化');const stat=await lstat(path);if(!stat.isFile()||stat.size>4*1024*1024||total+stat.size>32*1024*1024)throw Error('部署包文件大小无效');const data=await readFile(path);total+=data.length;if(data.length>4*1024*1024||total>32*1024*1024||sha(data)!==hash)throw Error('部署包校验失败');files.set(name,data)}return files
}
