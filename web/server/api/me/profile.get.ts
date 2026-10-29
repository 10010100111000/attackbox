import { defineEventHandler, createError } from 'h3'
import { eq, sql } from 'drizzle-orm'
import { requireLearner } from '../../utils/learner'
import { db } from '../../db'
import { users } from '../../db/schema'
export default defineEventHandler(async event => {
  const owner = requireLearner(event)
  const [account] = await db
    .select({ id: users.id, hasPassword: sql<boolean>`${users.passwordHash} is not null` })
    .from(users)
    .where(eq(users.id, owner))
  if (!account) throw createError({ statusCode: 404, message: '账户不存在' })
  return { data: account }
})
