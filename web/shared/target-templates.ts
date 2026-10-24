export interface TargetPort { port:number; protocol:'tcp'|'udp' }
export interface TargetTemplateInput { name:string; description:string; image:string; sourceUrl:string; ports:TargetPort[]; accessInstructions:string; enabled:boolean }
export interface TargetTemplate extends TargetTemplateInput { id:string; lockVersion:number; createdAt:string; updatedAt:string }
export interface TargetTemplateList { items:TargetTemplate[]; total:number; page:number; pageSize:number }
export class TemplateValidationError extends Error { field:string; constructor(field:string,message:string){super(message);this.field=field} }
export function parseTargetTemplate(value:unknown):TargetTemplateInput {
 const fail=(field:string,message:string):never=>{throw new TemplateValidationError(field,message)}
 if(!value||typeof value!=='object'||Array.isArray(value))return fail('form','模板格式无效')
 const v=value as Record<string,unknown>
 const text=(field:string,max:number,required=false)=>{const x=v[field];if(typeof x!=='string'||x.trim().length>max||required&&!x.trim())return fail(field,`请填写有效内容（最多 ${max} 字符）`);return x.trim()}
 const name=text('name',120,true),description=text('description',500),image=text('image',512,true),sourceUrl=text('sourceUrl',1000),accessInstructions=text('accessInstructions',4000)
 // Explicitly scoped Docker reference subset: lowercase repository, optional registry/port,
 // required tag or sha256 digest. Credentials, URLs and arbitrary runtime flags are excluded.
 const match=image.match(/^(.+?)(?:@sha256:([a-f0-9]{64})|:([A-Za-z0-9_][A-Za-z0-9_.-]{0,127}))$/)
 if(!match)return fail('image','请使用明确的镜像 tag 或 sha256 digest，例如 nginx:1.27')
 const parts=match[1]!.split('/'),component=/^[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*$/
 if(parts.length>1&&(parts[0]!.includes('.')||parts[0]!.includes(':')||parts[0]==='localhost')){
  const registry=parts.shift()!;const m=registry.match(/^([a-z0-9]+(?:[.-][a-z0-9]+)*)(?::([0-9]{1,5}))?$/)
  if(!m||m[2]&&(Number(m[2])<1||Number(m[2])>65535))return fail('image','镜像仓库地址格式无效')
 }
 if(!parts.length||!parts.every(p=>component.test(p))||match[1]!.length>255)return fail('image','镜像名称格式无效')
 if(sourceUrl){try{const u=new URL(sourceUrl);if(!['https:','http:'].includes(u.protocol)||u.username||u.password)throw Error()}catch{return fail('sourceUrl','来源链接需为不含账号密码的 HTTP/HTTPS 地址')}}
 if(typeof v.enabled!=='boolean')return fail('enabled','请选择模板启用状态')
 if(!Array.isArray(v.ports)||v.ports.length>20)return fail('ports','最多配置 20 个服务端口')
 const ports:TargetPort[]=[],seen=new Set<string>()
 for(const item of v.ports){if(!item||typeof item!=='object'||!Number.isInteger(item.port)||item.port<1||item.port>65535||!['tcp','udp'].includes(item.protocol))return fail('ports','端口需为 1–65535 的整数，协议为 TCP 或 UDP');const key=`${item.port}/${item.protocol}`;if(seen.has(key))return fail('ports','同一协议下不能重复配置端口');seen.add(key);ports.push({port:item.port,protocol:item.protocol})}
 return {name,description,image,sourceUrl,ports,accessInstructions,enabled:v.enabled}
}
