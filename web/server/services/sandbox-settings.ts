import { eq, and } from 'drizzle-orm'
import { createError } from 'h3'
import { db } from '../db'
import { sandboxNodeSettings } from '../db/schema'
import { sandboxHub } from '../sandbox'
import { parseNodeSettings } from '../sandbox/settings'
export async function loadSandboxSettings() {
  if (!sandboxHub().config.enabled) return
  const records = await db.select().from(sandboxNodeSettings)
  for (const row of records) sandboxHub().applySettings(row.nodeId, parseNodeSettings(row))
}
export async function saveSandboxSettings(nodeId: string, body: unknown, actorId: string) {
  if (!sandboxHub().config.nodes.some(n => n.id === nodeId))
    throw createError({ statusCode: 404, message: '节点不存在' })
  let settings
  try {
    settings = parseNodeSettings(body)
  } catch (error) {
    throw createError({ statusCode: 400, message: (error as Error).message })
  }
  const values = {
    ...settings,
    lockVersion: settings.lockVersion + 1,
    updatedBy: actorId,
    updatedAt: new Date()
  }
  const records =
    settings.lockVersion === 0
      ? await db
          .insert(sandboxNodeSettings)
          .values({ nodeId, ...values })
          .onConflictDoNothing()
          .returning()
      : await db
          .update(sandboxNodeSettings)
          .set(values)
          .where(
            and(
              eq(sandboxNodeSettings.nodeId, nodeId),
              eq(sandboxNodeSettings.lockVersion, settings.lockVersion)
            )
          )
          .returning()
  if (!records[0])
    throw createError({ statusCode: 409, message: '设置已被修改，请刷新节点后重新编辑' })
  // Version checks in the hub also prevent slow older saves replacing newer settings.
  sandboxHub().applySettings(nodeId, parseNodeSettings(records[0]))
  return { ...settings, lockVersion: records[0].lockVersion }
}
