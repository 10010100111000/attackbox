import { pgTable, varchar, jsonb, integer, timestamp, uuid, check } from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { users } from './auth'
import type { LearnHome } from '../../../shared/learn-home'
export const learnHomeSettings = pgTable(
  'learn_home_settings',
  {
    id: varchar('id', { length: 16 }).primaryKey(),
    config: jsonb('config').$type<Omit<LearnHome, 'lockVersion'>>().notNull(),
    lockVersion: integer('lock_version').notNull(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
    updatedBy: uuid('updated_by').references(() => users.id, { onDelete: 'set null' })
  },
  t => [check('learn_home_singleton', sql`${t.id}='global' and ${t.lockVersion}>0`)]
)
