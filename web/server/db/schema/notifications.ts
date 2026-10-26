import {
  pgTable,
  uuid,
  text,
  varchar,
  timestamp,
  primaryKey,
  index,
} from 'drizzle-orm/pg-core'
import { users } from './auth'
export const announcements = pgTable(
  'announcements',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    authorId: uuid('author_id')
      .notNull()
      .references(() => users.id, { onDelete: 'restrict' }),
    title: varchar('title', { length: 120 }).notNull(),
    body: text('body').notNull(),
    link: varchar('link', { length: 500 }),
    publishedAt: timestamp('published_at', { withTimezone: true })
      .notNull()
      .defaultNow(),
    withdrawnAt: timestamp('withdrawn_at', { withTimezone: true }),
  },
  (t) => [index('announcements_published_idx').on(t.publishedAt)],
)
export const notificationReads = pgTable(
  'notification_reads',
  {
    announcementId: uuid('announcement_id')
      .notNull()
      .references(() => announcements.id, { onDelete: 'cascade' }),
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    readAt: timestamp('read_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    primaryKey({ columns: [t.announcementId, t.userId] }),
    index('notification_reads_user_idx').on(t.userId),
  ],
)
