import { and, lte, ne } from 'drizzle-orm'
import { db } from '../db'
import { targetInstances } from '../db/schema'
import { targetOperation } from './target-runtime'

// The Worker enforces expiry. This loop reconciles DB state even when no learner
// visits the page; offline or failed cleanup must never be reported as stopped.
export async function reconcileTargetLeases() {
  const rows = await db
    .select({ id: targetInstances.id, userId: targetInstances.userId })
    .from(targetInstances)
    .where(and(ne(targetInstances.state, 'stopped'), lte(targetInstances.expiresAt, new Date())))
  for (let i = 0; i < rows.length; i += 4) {
    await Promise.allSettled(
      rows.slice(i, i + 4).map(row => targetOperation(row.userId, { id: row.id }, 'stop'))
    )
  }
}
