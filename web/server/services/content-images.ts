import {and,eq,or,inArray} from 'drizzle-orm'
import {db} from '../db'
import {moduleImages,chapterAssets,modules,assets} from '../db/schema'
import {learningOutline} from './learning'
import {createError} from 'h3'
type Tx=Parameters<Parameters<typeof db.transaction>[0]>[0]
export async function contentImageUrls(tx:Tx,moduleId:string,chapterId?:string,admin=false){
 const refs=chapterId?await tx.select({id:chapterAssets.assetId}).from(chapterAssets).where(eq(chapterAssets.chapterId,chapterId)):[]
 const rows=await tx.select().from(moduleImages).where(and(eq(moduleImages.moduleId,moduleId),admin?undefined:chapterId?or(eq(moduleImages.public,true),inArray(moduleImages.assetId,refs.map(r=>r.id))):eq(moduleImages.public,true)))
 const [mod]=await tx.select({slug:modules.slug}).from(modules).where(eq(modules.id,moduleId))
 return Object.fromEntries(rows.map(r=>[r.href,`/api/modules/${mod!.slug}/assets/${r.assetId}`]))
}
export async function readableContentAsset(slug:string,id:string,userId?:string,admin=false){return db.transaction(async tx=>{
 const [mod]=await tx.select().from(modules).where(and(eq(modules.slug,slug),admin?undefined:eq(modules.status,'published')))
 if(!mod)throw createError({statusCode:404,message:'资源不存在'})
 const rows=await tx.select({asset:assets,public:moduleImages.public}).from(moduleImages).innerJoin(assets,eq(moduleImages.assetId,assets.id)).where(and(eq(moduleImages.moduleId,mod.id),eq(moduleImages.assetId,id)))
 if(!rows.length)throw createError({statusCode:404,message:'资源不存在'})
 if(!admin&&!rows.some(r=>r.public)){
  const outline=await learningOutline(tx,mod.id,userId),allowed=outline.filter(c=>!c.isLocked).map(c=>c.id)
  const refs=allowed.length?await tx.select().from(chapterAssets).where(and(eq(chapterAssets.assetId,id),inArray(chapterAssets.chapterId,allowed))):[]
  if(!refs.length)throw createError({statusCode:userId?403:401,message:'没有读取该章节资源的权限'})
 }
 return rows[0]!.asset
},{isolationLevel:'repeatable read',accessMode:'read only'})}
