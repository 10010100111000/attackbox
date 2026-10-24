import {createError} from 'h3'
import {and,eq,sql,desc} from 'drizzle-orm'
import {db} from '../db'
import {targetTemplates as templates} from '../db/schema'
import {parseTargetTemplate,TemplateValidationError} from '../../shared/target-templates'
function id(value:string){if(!/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(value))throw createError({statusCode:400,message:'模板标识无效'});return value}
export async function listTargetTemplates(query:Record<string,unknown>){
 const page=Number(query.page||1),search=typeof query.search==='string'?query.search.trim().slice(0,100):''
 if(!Number.isSafeInteger(page)||page<1||page>100000)throw createError({statusCode:400,message:'页码无效'})
 const where=search?sql`strpos(lower(${templates.name} || ' ' || ${templates.image}),lower(${search}))>0`:undefined
 return db.transaction(async tx=>{const [total]=await tx.select({count:sql<number>`count(*)::int`}).from(templates).where(where);const items=await tx.select().from(templates).where(where).orderBy(desc(templates.updatedAt),templates.id).limit(20).offset((page-1)*20);return {items,total:total!.count,page,pageSize:20}},{isolationLevel:'repeatable read',accessMode:'read only'})
}
export async function getTargetTemplate(value:string){const [record]=await db.select().from(templates).where(eq(templates.id,id(value)));if(!record)throw createError({statusCode:404,message:'模板不存在'});return record}
export async function saveTargetTemplate(value:string|null,body:unknown){
 let input
 try{input=parseTargetTemplate(body)}catch(e){if(e instanceof TemplateValidationError)throw createError({statusCode:400,message:e.message,data:{field:e.field}});throw e}
 if(!value)return (await db.insert(templates).values(input).returning())[0]!
 const version=(body as Record<string,unknown>).lockVersion
 if(typeof version!=='number'||!Number.isSafeInteger(version)||version<1||version>=2147483647)throw createError({statusCode:400,message:'缺少有效修改标识，请重新打开模板'})
 const [saved]=await db.update(templates).set({...input,lockVersion:version+1,updatedAt:new Date()}).where(and(eq(templates.id,id(value)),eq(templates.lockVersion,version))).returning()
 if(!saved){await getTargetTemplate(value);throw createError({statusCode:409,message:'模板已被其他管理员修改；当前输入已保留，请重新读取后合并修改'})}
 return saved
}
