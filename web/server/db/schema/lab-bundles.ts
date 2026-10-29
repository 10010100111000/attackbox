import { pgTable, uuid, text, jsonb, timestamp } from 'drizzle-orm/pg-core'
import { users } from './auth'
import type { LabGitSource } from '../../../shared/compose-labs'
export const labBundles = pgTable('lab_bundles', {
  source: jsonb('source').$type<Omit<LabGitSource, 'hasToken'>>(),
  gitTokenCipher: text('git_token_cipher'),
  id: uuid('id').primaryKey().defaultRandom(),
  actorId: uuid('actor_id')
    .notNull()
    .references(() => users.id, { onDelete: 'restrict' }),
  hash: text('hash').notNull(),
  manifest: jsonb('manifest').$type<Record<string, string>>().notNull(),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow()
})
