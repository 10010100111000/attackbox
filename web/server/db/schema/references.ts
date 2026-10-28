import { pgTable, uuid, varchar, text, integer, timestamp, jsonb, uniqueIndex, check, primaryKey } from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { users } from './auth'
import { assets } from './assets'
export interface ReferenceManifest { md: string; images: Record<string, { path: string; sha: string }> }
export const referenceDocuments = pgTable('reference_documents', {
  id: uuid('id').primaryKey().defaultRandom(),
  slug: varchar('slug', { length: 100 }).notNull(),
  title: varchar('title', { length: 160 }).notNull(),
  summary: varchar('summary', { length: 500 }).notNull().default(''),
  bodyMarkdown: text('body_markdown').notNull(),
  status: varchar('status', { length: 20 }).$type<'unpublished' | 'published'>().notNull().default('unpublished'),
  sourceOrigin: text('source_origin').notNull(), sourcePath: text('source_path').notNull(), sourceCommit: text('source_commit').notNull(),
  manifest: jsonb('manifest').$type<ReferenceManifest>().notNull(),
  lockVersion: integer('lock_version').notNull().default(1),
  createdBy: uuid('created_by').notNull().references(() => users.id, { onDelete: 'restrict' }),
  updatedAt: timestamp('updated_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
  publishedAt: timestamp('published_at', { withTimezone: true, mode: 'date' }),
}, t => [
  uniqueIndex('references_slug_unique').on(t.slug), uniqueIndex('references_source_unique').on(t.sourceOrigin, t.sourcePath),
  check('references_status_check', sql`${t.status} in ('unpublished','published')`),
  check('references_version_check', sql`${t.lockVersion} > 0`),
])
export const referenceImages = pgTable('reference_images', {
  referenceId: uuid('reference_id').notNull().references(() => referenceDocuments.id, { onDelete: 'cascade' }),
  href: text('href').notNull(), assetId: uuid('asset_id').notNull().references(() => assets.id, { onDelete: 'restrict' }),
}, t => [primaryKey({ columns: [t.referenceId, t.href] })])
