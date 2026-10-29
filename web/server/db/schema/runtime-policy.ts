import { pgTable, varchar, integer, timestamp, uuid, check } from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { users } from './auth'
export const runtimePolicies = pgTable(
  'runtime_policies',
  {
    id: varchar('id', { length: 16 }).primaryKey(),
    sandboxTtlMinutes: integer('sandbox_ttl_minutes').notNull(),
    sandboxIdleMinutes: integer('sandbox_idle_minutes').notNull(),
    targetTtlMinutes: integer('target_ttl_minutes').notNull(),
    lockVersion: integer('lock_version').notNull(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
    updatedBy: uuid('updated_by').references(() => users.id, {
      onDelete: 'set null'
    })
  },
  t => [
    check('runtime_policy_singleton', sql`${t.id}='global'`),
    check(
      'runtime_policy_limits',
      sql`${t.sandboxTtlMinutes} between 1 and 1440 and ${t.sandboxIdleMinutes} between 1 and ${t.sandboxTtlMinutes} and ${t.targetTtlMinutes} between 1 and 1440 and ${t.lockVersion}>0`
    )
  ]
)
