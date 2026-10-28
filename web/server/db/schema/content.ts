import { pgTable, uuid, varchar, text, integer, timestamp, uniqueIndex, index, check, primaryKey } from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { users } from './auth'
import { targetTemplates } from './target-templates'
import { assets } from './assets'
import { contentCategories } from './categories'
import type { Normalization, Difficulty } from '../../../shared/content'

// One editable record per module. Publishing controls visibility only.
export const modules = pgTable('modules', {
  id: uuid('id').primaryKey().defaultRandom(),
  slug: varchar('slug', { length: 100 }).notNull(),
  title: varchar('title', { length: 160 }).notNull(),
  defaultTargetTemplateId: uuid('default_target_template_id').references(() => targetTemplates.id, { onDelete: 'restrict' }),
  summary: varchar('summary', { length: 500 }).notNull().default(''),
  descriptionMarkdown: text('description_markdown').notNull().default(''),
  learningOutcomes: text('learning_outcomes').array().notNull().default(sql`'{}'::text[]`),
  difficulty: varchar('difficulty', { length: 20 }).$type<Difficulty>().notNull().default('beginner'),
  categoryId: uuid('category_id').references(() => contentCategories.id, { onDelete: 'restrict' }),
  coverAssetId: uuid('cover_asset_id').references(() => assets.id, { onDelete: 'set null' }),
  status: varchar('status', { length: 20 }).$type<'unpublished' | 'published'>().notNull().default('unpublished'),
  lockVersion: integer('lock_version').notNull().default(1),
  createdBy: uuid('created_by').notNull().references(() => users.id, { onDelete: 'restrict' }),
  createdAt: timestamp('created_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
  publishedAt: timestamp('published_at', { withTimezone: true, mode: 'date' }),
}, table => [
  uniqueIndex('modules_slug_unique_idx').on(table.slug),
  index('modules_status_idx').on(table.status),
  check('modules_slug_check', sql`${table.slug} ~ '^[a-z0-9]+(-[a-z0-9]+)*$'`),
  check('modules_status_check', sql`${table.status} in ('unpublished', 'published')`),
  check('modules_difficulty_check', sql`${table.difficulty} in ('beginner', 'intermediate', 'advanced')`),
  check('modules_lock_version_check', sql`${table.lockVersion} > 0`),
])

export const chapters = pgTable('chapters', {
  id: uuid('id').primaryKey().defaultRandom(),
  moduleId: uuid('module_id').notNull().references(() => modules.id, { onDelete: 'cascade' }),
  slug: varchar('slug', { length: 100 }).notNull(),
  position: integer('position').notNull(),
  title: varchar('title', { length: 160 }).notNull(),
  durationMinutes: integer('duration_minutes'),
  targetMode: varchar('target_mode', {length:16}).$type<'inherit'|'none'|'custom'>().notNull().default('inherit'),
  targetTemplateId: uuid('target_template_id').references(() => targetTemplates.id, {onDelete:'restrict'}),
  access: varchar('access', { length: 20 }).$type<'public' | 'authenticated'>().notNull().default('public'),
  bodyMarkdown: text('body_markdown').notNull().default(''),
  createdAt: timestamp('created_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
}, table => [
  uniqueIndex('chapters_module_slug_unique').on(table.moduleId, table.slug),
  uniqueIndex('chapters_module_position_unique').on(table.moduleId, table.position),
  check('chapters_slug_check', sql`${table.slug} ~ '^[a-z0-9]+(-[a-z0-9]+)*$'`),
  check('chapters_position_check', sql`${table.position} > 0`),
  check('chapters_target_check', sql`(${table.targetMode}='custom' and ${table.targetTemplateId} is not null) or (${table.targetMode} in ('inherit','none') and ${table.targetTemplateId} is null)`),
  check('chapters_duration_check', sql`${table.durationMinutes} is null or ${table.durationMinutes} between 1 and 1440`),
  check('chapters_access_check', sql`${table.access} in ('public', 'authenticated')`),
])

export const chapterQuestions = pgTable('chapter_questions', {
  id: uuid('id').primaryKey().defaultRandom(),
  chapterId: uuid('chapter_id').notNull().references(() => chapters.id, { onDelete: 'cascade' }),
  position: integer('position').notNull(),
  promptMarkdown: text('prompt_markdown').notNull(),
  errorHint: text('error_hint'),
  normalization: varchar('normalization', { length: 20 }).$type<Normalization>().notNull().default('trim'),
  answer: text('answer'),
}, table => [
  uniqueIndex('chapter_questions_position_unique').on(table.chapterId, table.position),
  check('chapter_questions_normalization_check', sql`${table.normalization} in ('exact', 'trim', 'trim_casefold')`),
  check('chapter_questions_position_check', sql`${table.position} > 0`),
])

export const chapterAssets = pgTable('chapter_assets', {
  chapterId: uuid('chapter_id').notNull().references(() => chapters.id, { onDelete: 'cascade' }),
  assetId: uuid('asset_id').notNull().references(() => assets.id, { onDelete: 'restrict' }),
}, table => [primaryKey({ columns: [table.chapterId, table.assetId] }), index('chapter_assets_asset_idx').on(table.assetId)])

// Ordered recommendations. They do not gate access to a module.
export const modulePrerequisites = pgTable('module_prerequisites', {
  moduleId: uuid('module_id').notNull().references(() => modules.id, { onDelete: 'cascade' }),
  prerequisiteId: uuid('prerequisite_id').notNull().references(() => modules.id, { onDelete: 'cascade' }),
  position: integer('position').notNull(),
}, table => [
  primaryKey({ columns: [table.moduleId, table.prerequisiteId] }),
  uniqueIndex('module_prerequisites_position_unique').on(table.moduleId, table.position),
  check('module_prerequisites_self_check', sql`${table.moduleId} <> ${table.prerequisiteId}`),
  check('module_prerequisites_position_check', sql`${table.position} > 0`),
])
