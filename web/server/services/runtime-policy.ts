import { and, eq } from 'drizzle-orm'
import { createError } from 'h3'
import { db } from '../db'
import { runtimePolicies } from '../db/schema'
import { defaultRuntimePolicy, parseRuntimePolicy } from '../../shared/runtime-policy'
export async function getRuntimePolicy() {
  const [row] = await db.select().from(runtimePolicies).where(eq(runtimePolicies.id, 'global'))
  return row ? parseRuntimePolicy(row) : { ...defaultRuntimePolicy }
}
export async function saveRuntimePolicy(body: unknown, actor: string) {
  let policy
  try {
    policy = parseRuntimePolicy(body)
  } catch (e) {
    throw createError({ statusCode: 400, message: (e as Error).message })
  }
  const values = {
    ...policy,
    lockVersion: policy.lockVersion + 1,
    updatedAt: new Date(),
    updatedBy: actor
  }
  const rows =
    policy.lockVersion === 0
      ? await db
          .insert(runtimePolicies)
          .values({ id: 'global', ...values })
          .onConflictDoNothing()
          .returning()
      : await db
          .update(runtimePolicies)
          .set(values)
          .where(
            and(
              eq(runtimePolicies.id, 'global'),
              eq(runtimePolicies.lockVersion, policy.lockVersion)
            )
          )
          .returning()
  if (!rows[0])
    throw createError({
      statusCode: 409,
      message: '策略已被其他管理员修改，请重新载入后再保存'
    })
  return parseRuntimePolicy(rows[0])
}
