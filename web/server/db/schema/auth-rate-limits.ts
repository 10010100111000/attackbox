import { varchar, integer, timestamp, index, check, pgTable } from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'

export const authRateLimits = pgTable(
  'auth_rate_limits',
  {
    key: varchar('key', { length: 64 }).primaryKey(),
    attempts: integer('attempts').notNull(),
    windowAt: timestamp('window_at', { withTimezone: true }).notNull()
  },
  table => [
    index('auth_rate_limits_window_idx').on(table.windowAt),
    check('auth_rate_limits_attempts_check', sql`${table.attempts} > 0`)
  ]
)
