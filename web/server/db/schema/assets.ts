import {
  pgTable,
  uuid,
  varchar,
  text,
  bigint,
  timestamp,
  uniqueIndex,
  index,
  check
} from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { bytea } from './types'
import { users } from './auth'

export const assets = pgTable(
  'assets',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    storageKey: text('storage_key').notNull(),
    originalName: varchar('original_name', { length: 255 }).notNull(),
    mediaType: varchar('media_type', { length: 120 }).notNull(),
    byteSize: bigint('byte_size', { mode: 'number' }).notNull(),
    sha256: bytea('sha256').notNull(),
    visibility: varchar('visibility', { length: 16 }).notNull(),
    uploadedBy: uuid('uploaded_by')
      .notNull()
      .references(() => users.id, { onDelete: 'restrict' }),
    createdAt: timestamp('created_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
    archivedAt: timestamp('archived_at', { withTimezone: true, mode: 'date' })
  },
  table => [
    uniqueIndex('assets_storage_key_unique_idx').on(table.storageKey),
    index('assets_sha256_idx').on(table.sha256),
    index('assets_uploaded_by_idx').on(table.uploadedBy),
    check('assets_visibility_check', sql`${table.visibility} in ('public', 'private')`),
    check('assets_byte_size_check', sql`${table.byteSize} >= 0`),
    check('assets_sha256_len_check', sql`octet_length(${table.sha256}) = 32`)
  ]
)
