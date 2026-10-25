import {asc,eq,sql} from 'drizzle-orm'
import {createError} from 'h3'
import {db} from '../db'
import {contentCategories,modules} from '../db/schema'
export async function adminCategories(){
 return db.select({id:contentCategories.id,name:contentCategories.name,slug:contentCategories.slug,position:contentCategories.position,moduleCount:sql<number>`count(${modules.id})::int`}).from(contentCategories).leftJoin(modules,eq(modules.categoryId,contentCategories.id)).groupBy(contentCategories.id).orderBy(asc(contentCategories.position),asc(contentCategories.name))
}
export async function editCategory(id:string,currentName:unknown,name:unknown){
 if(!/^[a-f0-9-]{36}$/.test(id))throw createError({statusCode:400,message:'分类标识无效'})
 const next=typeof name==='string'?name.trim():''
 if(!next||next.length>80)throw createError({statusCode:400,message:'分类名称需要 1–80 字'})
 try{
  return await db.transaction(async tx=>{
   const [row]=await tx.select().from(contentCategories).where(eq(contentCategories.id,id)).for('update')
   if(!row)throw createError({statusCode:404,message:'分类已删除，请刷新列表'})
   if(row.name!==currentName)throw createError({statusCode:409,message:'分类已被修改，请刷新列表'})
   const [updated]=await tx.update(contentCategories).set({name:next}).where(eq(contentCategories.id,id)).returning()
   return updated
  })
 }catch(e:any){if(e?.code==='23505'||e?.cause?.code==='23505')throw createError({statusCode:409,message:'分类名称已存在'});throw e}
}
export async function deleteCategory(id:string,currentName:unknown,expectedCount:unknown){
 if(!/^[a-f0-9-]{36}$/.test(id))throw createError({statusCode:400,message:'分类标识无效'})
 if(!Number.isSafeInteger(expectedCount)||Number(expectedCount)<0)throw createError({statusCode:400,message:'请先刷新分类及引用数量'})
 return db.transaction(async tx=>{
  const [row]=await tx.select().from(contentCategories).where(eq(contentCategories.id,id)).for('update')
  if(!row)throw createError({statusCode:404,message:'分类已删除，请刷新列表'})
  if(row.name!==currentName)throw createError({statusCode:409,message:'分类已被修改，请刷新列表'})
  const affected=await tx.select({id:modules.id}).from(modules).where(eq(modules.categoryId,id)).for('update')
  if(affected.length!==expectedCount)throw createError({statusCode:409,message:'分类引用数量已变化，请返回列表刷新后重新确认'})
  const updated=await tx.update(modules).set({categoryId:null,lockVersion:sql`${modules.lockVersion}+1`,updatedAt:new Date()}).where(eq(modules.categoryId,id)).returning({id:modules.id,lockVersion:modules.lockVersion})
  await tx.delete(contentCategories).where(eq(contentCategories.id,id))
  return {id,affectedModules:updated.map(m=>({...m,previousVersion:m.lockVersion-1}))}
 })
}
