import { defineEventHandler } from 'h3'
import { inArray } from 'drizzle-orm'
import { requireAdmin } from '../../../utils/admin'
import { sandboxController } from '../../../sandbox'
import { db } from '../../../db'
import { users } from '../../../db/schema'
export default defineEventHandler(async event => {
  requireAdmin(event)
  const list = sandboxController().list()
  const ids = [...new Set(list.map(r => r.userId))]
  const owners = ids.length
    ? await db
        .select({ id: users.id, email: users.email, displayName: users.displayName })
        .from(users)
        .where(inArray(users.id, ids))
    : []
  const byId = new Map(owners.map(u => [u.id, u]))
  return {
    ok: true,
    data: list.map(r => ({
      ...r,
      email: byId.get(r.userId)?.email || null,
      displayName: byId.get(r.userId)?.displayName || null
    }))
  }
})
