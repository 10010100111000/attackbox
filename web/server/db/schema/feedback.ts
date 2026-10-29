import { pgTable, uuid, varchar, text, timestamp, index, check } from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { users } from './auth'
import type { FeedbackCategory, FeedbackStatus } from '../../../shared/feedback'

export const feedback = pgTable(
  'feedback',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    userId: uuid('user_id').references(() => users.id, { onDelete: 'set null' }),
    category: varchar('category', { length: 16 }).$type<FeedbackCategory>().notNull(),
    body: text('body').notNull(),
    sourcePath: varchar('source_path', { length: 500 }),
    status: varchar('status', { length: 16 }).$type<FeedbackStatus>().notNull().default('new'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow()
  },
  table => [
    index('feedback_status_created_idx').on(table.status, table.createdAt),
    index('feedback_user_created_idx').on(table.userId, table.createdAt),
    check('feedback_category_check', sql`${table.category} in ('bug', 'content', 'suggestion')`),
    check('feedback_status_check', sql`${table.status} in ('new', 'in_progress', 'resolved')`),
    check('feedback_body_length_check', sql`length(btrim(${table.body})) between 1 and 4000`)
  ]
)
