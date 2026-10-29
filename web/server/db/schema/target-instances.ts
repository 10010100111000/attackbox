import {
  pgTable,
  uuid,
  text,
  integer,
  jsonb,
  timestamp,
  unique,
  index,
  check
} from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { users } from './auth'
export const targetInstances = pgTable(
  'target_instances',
  {
    deploymentId: text('deployment_id').notNull(),
    expiresAt: timestamp('expires_at', { withTimezone: true }).notNull(),
    id: text('id').primaryKey(),
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'restrict' }),
    scope: text('scope').notNull(),
    nodeId: text('node_id').notNull(),
    bootId: text('boot_id').notNull(),
    state: text('state').notNull(),
    result: jsonb('result').$type<Record<string, unknown>>(),
    revision: integer('revision').notNull().default(0),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow()
  },
  t => [
    check('target_instances_deployment_check', sql`${t.deploymentId} ~ '^dep_[a-f0-9]{64}$'`),
    index('target_instances_expiry_idx').on(t.expiresAt),
    unique().on(t.userId, t.scope)
  ]
)
