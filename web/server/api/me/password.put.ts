import { defineEventHandler, readBody, createError } from 'h3'
import { db } from '../../db'
import { users, authSessions, emailTokens } from '../../db/schema'
import { and, eq, ne, isNull, gt } from 'drizzle-orm'
import { requireLearner } from '../../utils/learner'
import { hashPassword, verifyPassword } from '../../utils/auth'
export default defineEventHandler(async event=>{
 const owner=requireLearner(event), session=event.context.session
 if(!session)throw createError({statusCode:401,message:'请重新登录'})
 const body=await readBody(event), current=body?.currentPassword, password=body?.newPassword
 if(typeof current!=='string'||!current||current.length>1024||typeof password!=='string'||password.length<6||password.length>1024)
  throw createError({statusCode:400,message:'请填写当前密码，新密码需为 6–1024 位'})
 const [user]=await db.select().from(users).where(eq(users.id,owner))
 if(!user?.passwordHash)throw createError({statusCode:409,message:'此账户未设置本地密码'})
 if(!await verifyPassword(user.passwordHash,current))throw createError({statusCode:400,message:'当前密码不正确'})
 const passwordHash=await hashPassword(password), now=new Date()
 await db.transaction(async tx=>{
  const [locked]=await tx.select().from(users).where(eq(users.id,owner)).for('update')
  if(!locked||locked.status!=='active'||locked.passwordHash!==user.passwordHash)throw createError({statusCode:409,message:'账户状态已变化，请重新登录后重试'})
  const [live]=await tx.select().from(authSessions).where(and(eq(authSessions.id,session.id),eq(authSessions.userId,owner),isNull(authSessions.revokedAt),gt(authSessions.expiresAt,new Date()))).for('update')
  if(!live)throw createError({statusCode:401,message:'登录已失效，请重新登录'})
  await tx.update(users).set({passwordHash,updatedAt:now}).where(eq(users.id,owner))
  await tx.update(authSessions).set({revokedAt:now}).where(and(eq(authSessions.userId,owner),ne(authSessions.id,session.id),isNull(authSessions.revokedAt)))
  await tx.update(emailTokens).set({consumedAt:now}).where(and(eq(emailTokens.userId,owner),eq(emailTokens.purpose,'reset_password'),isNull(emailTokens.consumedAt)))
 })
 return {saved:true}
})
