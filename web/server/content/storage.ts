import path from 'node:path'
import {mkdir,writeFile,readFile,rm} from 'node:fs/promises'
import {createHash} from 'node:crypto'
import {createError} from 'h3'
export function contentDataDirectory(){return path.resolve(process.env.CONTENT_DATA_DIR||'/var/lib/attackbox-web/content')}
export function previewDirectory(id:string){if(!/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/.test(id))throw createError({statusCode:400,message:'预览标识无效'});return path.join(contentDataDirectory(),'previews',id)}
export async function writeSnapshotFile(directory:string,relative:string,body:Buffer){if(path.isAbsolute(relative)||relative.split('/').some(p=>!p||p==='.'||p==='..')||relative.includes('\\'))throw createError({statusCode:400,message:'仓库文件路径无效'});const file=path.join(directory,relative);await mkdir(path.dirname(file),{recursive:true,mode:0o700});await writeFile(file,body,{mode:0o600,flag:'wx'})}
export function hash(value:string|Buffer){return createHash('sha256').update(value).digest('hex')}
export async function storeImage(bytes:Buffer){const digest=hash(bytes);const directory=path.join(contentDataDirectory(),'assets');await mkdir(directory,{recursive:true,mode:0o700});const file=path.join(directory,digest);try{await writeFile(file,bytes,{mode:0o600,flag:'wx'})}catch(e:any){if(e.code!=='EEXIST'||hash(await readFile(file))!==digest)throw e}return digest}
export async function discardPreview(id:string){await rm(previewDirectory(id),{recursive:true,force:true})}
