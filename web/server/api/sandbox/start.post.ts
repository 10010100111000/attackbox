import { defineEventHandler, createError } from 'h3'
import { and, eq, ne, sql } from 'drizzle-orm'
import { db } from '../../db'
import { targetInstances } from '../../db/schema'
import { sandboxController } from '../../sandbox'
import { sandboxAction, requireSandboxUser, sandboxIdentity } from '../../utils/sandbox'
import { experimentNode } from '../../../shared/target-runtime'

export default defineEventHandler(event => sandboxAction(async () => {
  const userId = requireSandboxUser(event), controller = sandboxController()
  if (!controller.status(userId).available) throw createError({statusCode:503,message:'练习环境尚未开放'})
  const identity=await sandboxIdentity(userId)
  return db.transaction(async tx=>{
    // Share the same lock as target allocation: concurrent starts cannot place
    // the learner's sandbox and first target on different nodes.
    await tx.execute(sql`select pg_advisory_xact_lock(19842, hashtext(${userId}))`)
    const targets=await tx.select({nodeId:targetInstances.nodeId}).from(targetInstances).where(and(eq(targetInstances.userId,userId),ne(targetInstances.state,'stopped')))
    let preferred: string | null
    try {preferred=experimentNode(controller.status(userId).instance?.nodeId,targets.map(t=>t.nodeId))}
    catch {throw createError({statusCode:409,message:'已有实验环境分布在不同节点，请先停止冲突的靶机'})}
    return {ok:true,data:await controller.start(userId,identity,preferred)}
  })
}))
