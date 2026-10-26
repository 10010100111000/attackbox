import {createError} from 'h3'
import {and,eq,sql,desc} from 'drizzle-orm'
import {db} from '../db'
import {targetTemplates as templates} from '../db/schema'
import {publicTemplate} from './compose-templates'
function id(value:string){if(!/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(value))throw createError({statusCode:400,message:'模板标识无效'});return value}
export async function listTargetTemplates(query:Record<string,unknown>){
 const page=Number(query.page||1),search=typeof query.search==='string'?query.search.trim().slice(0,100):''
 if(!Number.isSafeInteger(page)||page<1||page>100000)throw createError({statusCode:400,message:'页码无效'})
 const where=search?sql`strpos(lower(${templates.name}),lower(${search}))>0`:undefined
 return db.transaction(async tx=>{const [total]=await tx.select({count:sql<number>`count(*)::int`}).from(templates).where(where);const items=await tx.select().from(templates).where(where).orderBy(desc(templates.updatedAt),templates.id).limit(20).offset((page-1)*20);return {items:items.map(publicTemplate),total:total!.count,page,pageSize:20}},{isolationLevel:'repeatable read',accessMode:'read only'})
}
export async function getTargetTemplate(value:string){const [record]=await db.select().from(templates).where(eq(templates.id,id(value)));if(!record)throw createError({statusCode:404,message:'模板不存在'});return publicTemplate(record)}
