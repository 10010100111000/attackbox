import { pgTable, uuid, bigserial, uniqueIndex, check, varchar, integer, boolean, timestamp } from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { users } from './auth'
// Worker protocol requires a stable positive int64; browser identities stay UUIDs.
export const sandboxIdentities = pgTable('sandbox_identities', {
  userId: uuid('user_id').primaryKey().references(() => users.id, { onDelete: 'cascade' }),
  workerUserId: bigserial('worker_user_id', { mode: 'number' }).notNull(),
}, table => [uniqueIndex('sandbox_identity_worker_id_idx').on(table.workerUserId), check('sandbox_identity_safe_integer_check', sql`${table.workerUserId} > 0 and ${table.workerUserId} <= 9007199254740991`)])

export const sandboxNodeSettings = pgTable('sandbox_node_settings', {
  nodeId: varchar('node_id', { length: 64 }).primaryKey(),
  label: varchar('label', { length: 80 }).notNull(),
  maxSlots: integer('max_slots').notNull(),
  ttlSeconds: integer('ttl_seconds').notNull(),
  idleSeconds: integer('idle_seconds').notNull(),
  memoryMb: integer('memory_mb').notNull(),
  pidsMax: integer('pids_max').notNull(),
  draining: boolean('draining').notNull().default(false),
  lockVersion: integer('lock_version').notNull().default(1),
  updatedAt: timestamp('updated_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
  updatedBy: uuid('updated_by').references(() => users.id, { onDelete: 'set null' }),
}, table => [
  check('sandbox_node_limits_check', sql`${table.maxSlots} between 1 and 250 and ${table.ttlSeconds} between 60 and 86400 and ${table.idleSeconds} between 60 and ${table.ttlSeconds} and ${table.memoryMb} between 128 and 65536 and ${table.pidsMax} between 32 and 4096`),
  check('sandbox_node_version_check', sql`${table.lockVersion} > 0`),
  check('sandbox_node_label_check', sql`length(btrim(${table.label})) > 0`),
])
