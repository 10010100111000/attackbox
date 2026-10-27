import { pgTable, uuid, text, varchar, timestamp, primaryKey, index, check } from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { users } from './auth'
export const notifications = pgTable('notifications', {
 id: uuid('id').primaryKey().defaultRandom(),
 kind: varchar('kind',{length:20}).$type<'announcement'|'message'>().notNull(),
 senderId: uuid('sender_id').references(()=>users.id,{onDelete:'set null'}),
 recipientId: uuid('recipient_id').references(()=>users.id,{onDelete:'cascade'}),
 title: varchar('title',{length:120}).notNull(),
 body: text('body').notNull(),
 link: varchar('link',{length:500}),
 publishedAt: timestamp('published_at',{withTimezone:true}).notNull().defaultNow(),
 withdrawnAt: timestamp('withdrawn_at',{withTimezone:true}),
},t=>[
 index('notifications_published_idx').on(t.publishedAt),
 index('notifications_recipient_idx').on(t.recipientId,t.publishedAt),
 check('notifications_recipient_check',sql`(${t.kind}='announcement' and ${t.recipientId} is null) or (${t.kind}='message' and ${t.recipientId} is not null)`),
])
export const notificationReceipts = pgTable('notification_receipts', {
 notificationId: uuid('notification_id').notNull().references(()=>notifications.id,{onDelete:'cascade'}),
 userId: uuid('user_id').notNull().references(()=>users.id,{onDelete:'cascade'}),
 readAt: timestamp('read_at',{withTimezone:true}),
 dismissedAt: timestamp('dismissed_at',{withTimezone:true}),
},t=>[primaryKey({columns:[t.notificationId,t.userId]}),index('notification_receipts_user_idx').on(t.userId)])
