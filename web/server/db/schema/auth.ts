import {
  pgTable,
  uuid,
  varchar,
  text,
  timestamp,
  uniqueIndex,
  index,
  check
} from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { bytea, inet } from './types'

export const users = pgTable(
  'users',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    email: varchar('email', { length: 254 }).notNull(),
    displayName: varchar('display_name', { length: 64 }).notNull(),
    passwordHash: text('password_hash'),
    role: varchar('role', { length: 16 }).notNull().default('user'),
    status: varchar('status', { length: 16 }).notNull().default('pending'),
    emailVerifiedAt: timestamp('email_verified_at', { withTimezone: true, mode: 'date' }),
    createdAt: timestamp('created_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
    lastLoginAt: timestamp('last_login_at', { withTimezone: true, mode: 'date' })
  },
  table => [
    uniqueIndex('users_email_unique_idx').on(table.email),
    check('users_email_lower_trimmed_check', sql`${table.email} = lower(btrim(${table.email}))`),
    check('users_display_name_non_empty_check', sql`length(btrim(${table.displayName})) > 0`),
    check('users_role_check', sql`${table.role} in ('user', 'admin')`),
    check('users_status_check', sql`${table.status} in ('pending', 'active', 'suspended')`)
  ]
)

export const authSessions = pgTable(
  'auth_sessions',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    tokenHash: bytea('token_hash').notNull(),
    createdAt: timestamp('created_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
    expiresAt: timestamp('expires_at', { withTimezone: true, mode: 'date' }).notNull(),
    lastSeenAt: timestamp('last_seen_at', { withTimezone: true, mode: 'date' })
      .notNull()
      .defaultNow(),
    revokedAt: timestamp('revoked_at', { withTimezone: true, mode: 'date' }),
    ip: inet('ip'),
    userAgent: varchar('user_agent', { length: 512 })
  },
  table => [
    uniqueIndex('auth_sessions_token_hash_unique_idx').on(table.tokenHash),
    index('auth_sessions_user_expires_idx').on(table.userId, table.expiresAt),
    check('auth_sessions_token_hash_len_check', sql`octet_length(${table.tokenHash}) = 32`),
    check('auth_sessions_expires_after_created_check', sql`${table.expiresAt} > ${table.createdAt}`)
  ]
)

export const emailTokens = pgTable(
  'email_tokens',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    userId: uuid('user_id').references(() => users.id, { onDelete: 'cascade' }),
    targetEmail: varchar('target_email', { length: 254 }).notNull(),
    purpose: varchar('purpose', { length: 32 }).notNull(),
    tokenHash: bytea('token_hash').notNull(),
    createdAt: timestamp('created_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
    expiresAt: timestamp('expires_at', { withTimezone: true, mode: 'date' }).notNull(),
    consumedAt: timestamp('consumed_at', { withTimezone: true, mode: 'date' })
  },
  table => [
    uniqueIndex('email_tokens_token_hash_unique_idx').on(table.tokenHash),
    index('email_tokens_email_purpose_created_idx').on(
      table.targetEmail,
      table.purpose,
      table.createdAt
    ),
    check(
      'email_tokens_reset_user_check',
      sql`${table.purpose} = 'verify_email' or ${table.userId} is not null`
    ),
    check(
      'email_tokens_purpose_check',
      sql`${table.purpose} in ('verify_email', 'reset_password')`
    ),
    check('email_tokens_token_hash_len_check', sql`octet_length(${table.tokenHash}) = 32`),
    check('email_tokens_expires_after_created_check', sql`${table.expiresAt} > ${table.createdAt}`)
  ]
)
