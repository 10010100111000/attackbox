import { pgTable, uuid, bigserial, uniqueIndex, check } from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { users } from './auth'
// Worker protocol requires a stable positive int64; browser identities stay UUIDs.
export const sandboxIdentities = pgTable('sandbox_identities', {
  userId: uuid('user_id').primaryKey().references(() => users.id, { onDelete: 'cascade' }),
  workerUserId: bigserial('worker_user_id', { mode: 'number' }).notNull(),
}, table => [uniqueIndex('sandbox_identity_worker_id_idx').on(table.workerUserId), check('sandbox_identity_safe_integer_check', sql`${table.workerUserId} > 0 and ${table.workerUserId} <= 9007199254740991`)])
