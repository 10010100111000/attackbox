import { and, eq } from 'drizzle-orm'
import { createError } from 'h3'
import { db } from '../db'
import { learnHomeSettings } from '../db/schema'
import {
  defaultLearnHome,
  parseLearnHome,
} from '../../shared/learn-home'
export async function getLearnHome() {
  const [row] = await db
    .select()
    .from(learnHomeSettings)
    .where(eq(learnHomeSettings.id, 'global'))
  return row ? parseLearnHome({ ...row.config, lockVersion: row.lockVersion }) : { ...defaultLearnHome }
}
export async function saveLearnHome(body: unknown, actor: string) {
  let policy
  try {
    policy = parseLearnHome(body)
  } catch (e) {
    throw createError({ statusCode: 400, message: (e as Error).message })
  }
  const values = {
    config: { enabled: policy.enabled, autoplay: policy.autoplay, intervalSeconds: policy.intervalSeconds, slides: policy.slides },
    lockVersion: policy.lockVersion + 1,
    updatedAt: new Date(),
    updatedBy: actor,
  }
  const rows =
    policy.lockVersion === 0
      ? await db
          .insert(learnHomeSettings)
          .values({ id: 'global', ...values })
          .onConflictDoNothing()
          .returning()
      : await db
          .update(learnHomeSettings)
          .set(values)
          .where(
            and(
              eq(learnHomeSettings.id, 'global'),
              eq(learnHomeSettings.lockVersion, policy.lockVersion),
            ),
          )
          .returning()
  if (!rows[0])
    throw createError({
      statusCode: 409,
      message: '首页已被其他管理员修改，请重新载入后再保存',
    })
  return parseLearnHome({ ...rows[0].config, lockVersion: rows[0].lockVersion })
}
