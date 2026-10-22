import {
  pgTable,
  uuid,
  varchar,
  text,
  integer,
  smallint,
  boolean,
  timestamp,
  uniqueIndex,
  index,
  check,
  primaryKey,
  foreignKey,
  unique,
} from 'drizzle-orm/pg-core';
import { sql } from 'drizzle-orm';
import { bytea } from './types';
import { users } from './auth';
import { assets } from './assets';

export const modules = pgTable(
  'modules',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    slug: varchar('slug', { length: 100 }).notNull(),
    publishedRevisionId: uuid('published_revision_id'),
    createdBy: uuid('created_by')
      .notNull()
      .references(() => users.id, { onDelete: 'restrict' }),
    createdAt: timestamp('created_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
    archivedAt: timestamp('archived_at', { withTimezone: true, mode: 'date' }),
  },
  (table) => [
    uniqueIndex('modules_slug_unique_idx').on(table.slug),
    index('modules_published_revision_idx').on(table.publishedRevisionId),
    check('modules_slug_format_check', sql`${table.slug} ~ '^[a-z0-9]+(-[a-z0-9]+)*$'`),
  ]
);

export const moduleRevisions = pgTable(
  'module_revisions',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    moduleId: uuid('module_id')
      .notNull()
      .references(() => modules.id, { onDelete: 'cascade' }),
    revisionNo: integer('revision_no').notNull(),
    state: varchar('state', { length: 16 }).notNull(),
    title: varchar('title', { length: 160 }),
    summary: varchar('summary', { length: 500 }),
    descriptionMarkdown: text('description_markdown'),
    coverAssetId: uuid('cover_asset_id').references(() => assets.id, { onDelete: 'set null' }),
    difficulty: varchar('difficulty', { length: 20 }).notNull().default('beginner'),
    lockVersion: integer('lock_version').notNull().default(1),
    createdBy: uuid('created_by')
      .notNull()
      .references(() => users.id, { onDelete: 'restrict' }),
    createdAt: timestamp('created_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
    publishedAt: timestamp('published_at', { withTimezone: true, mode: 'date' }),
  },
  (table) => [
    unique('module_revisions_module_rev_no_unique').on(table.moduleId, table.revisionNo),
    unique('module_revisions_id_module_id_unique').on(table.id, table.moduleId),
    uniqueIndex('module_revisions_single_draft_idx')
      .on(table.moduleId)
      .where(sql`${table.state} = 'draft'`),
    check('module_revisions_state_check', sql`${table.state} in ('draft', 'published')`),
    check('module_revisions_difficulty_check', sql`${table.difficulty} in ('beginner', 'intermediate', 'advanced')`),
    check('module_revisions_revision_no_check', sql`${table.revisionNo} > 0`),
    check('module_revisions_lock_version_check', sql`${table.lockVersion} >= 1`),
  ]
);

export const chapters = pgTable(
  'chapters',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    moduleId: uuid('module_id')
      .notNull()
      .references(() => modules.id, { onDelete: 'cascade' }),
    slug: varchar('slug', { length: 100 }).notNull(),
    createdAt: timestamp('created_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
    archivedAt: timestamp('archived_at', { withTimezone: true, mode: 'date' }),
  },
  (table) => [
    unique('chapters_module_slug_unique').on(table.moduleId, table.slug),
    unique('chapters_id_module_id_unique').on(table.id, table.moduleId),
    check('chapters_slug_format_check', sql`${table.slug} ~ '^[a-z0-9]+(-[a-z0-9]+)*$'`),
  ]
);

export const chapterRevisions = pgTable(
  'chapter_revisions',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    chapterId: uuid('chapter_id')
      .notNull()
      .references(() => chapters.id, { onDelete: 'cascade' }),
    title: varchar('title', { length: 160 }).notNull(),
    access: varchar('access', { length: 20 }).notNull().default('public'),
    bodyMarkdown: text('body_markdown').notNull().default(''),
    sourceKind: varchar('source_kind', { length: 20 }).notNull().default('editor'),
    sourceUrl: text('source_url'),
    sourceCommit: varchar('source_commit', { length: 64 }),
    sourceSha256: bytea('source_sha256'),
    isSealed: boolean('is_sealed').notNull().default(false),
    createdBy: uuid('created_by')
      .notNull()
      .references(() => users.id, { onDelete: 'restrict' }),
    createdAt: timestamp('created_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
  },
  (table) => [
    unique('chapter_revisions_id_chapter_id_unique').on(table.id, table.chapterId),
    index('chapter_revisions_chapter_sealed_idx').on(table.chapterId, table.isSealed),
    check('chapter_revisions_access_check', sql`${table.access} in ('public', 'authenticated')`),
    check('chapter_revisions_source_kind_check', sql`${table.sourceKind} in ('editor', 'upload', 'github')`),
    check(
      'chapter_revisions_sha256_len_check',
      sql`${table.sourceSha256} is null or octet_length(${table.sourceSha256}) = 32`
    ),
  ]
);

export const moduleRevisionChapters = pgTable(
  'module_revision_chapters',
  {
    moduleRevisionId: uuid('module_revision_id')
      .notNull()
      .references(() => moduleRevisions.id, { onDelete: 'cascade' }),
    moduleId: uuid('module_id').notNull(),
    chapterId: uuid('chapter_id')
      .notNull()
      .references(() => chapters.id, { onDelete: 'restrict' }),
    chapterRevisionId: uuid('chapter_revision_id')
      .notNull()
      .references(() => chapterRevisions.id, { onDelete: 'restrict' }),
    position: integer('position').notNull(),
  },
  (table) => [
    primaryKey({
      columns: [table.moduleRevisionId, table.chapterId],
      name: 'module_revision_chapters_pk',
    }),
    uniqueIndex('module_revision_chapters_pos_unique_idx').on(table.moduleRevisionId, table.position),
    check('module_revision_chapters_pos_check', sql`${table.position} > 0`),
    foreignKey({
      name: 'mrc_module_revision_fk',
      columns: [table.moduleRevisionId, table.moduleId],
      foreignColumns: [moduleRevisions.id, moduleRevisions.moduleId],
    }).onDelete('cascade'),
    foreignKey({
      name: 'mrc_chapter_fk',
      columns: [table.chapterId, table.moduleId],
      foreignColumns: [chapters.id, chapters.moduleId],
    }).onDelete('restrict'),
    foreignKey({
      name: 'mrc_chapter_revision_fk',
      columns: [table.chapterRevisionId, table.chapterId],
      foreignColumns: [chapterRevisions.id, chapterRevisions.chapterId],
    }).onDelete('restrict'),
  ]
);

export const chapterQuestions = pgTable(
  'chapter_questions',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    chapterRevisionId: uuid('chapter_revision_id')
      .notNull()
      .references(() => chapterRevisions.id, { onDelete: 'cascade' }),
    questionKey: uuid('question_key').notNull().defaultRandom(),
    position: integer('position').notNull(),
    promptMarkdown: text('prompt_markdown').notNull(),
    errorHint: text('error_hint'),
    normalization: varchar('normalization', { length: 20 }).notNull().default('trim'),
    answerHmac: bytea('answer_hmac'),
    answerKeyVersion: smallint('answer_key_version').notNull().default(1),
  },
  (table) => [
    uniqueIndex('chapter_questions_rev_pos_unique_idx').on(table.chapterRevisionId, table.position),
    uniqueIndex('chapter_questions_rev_key_unique_idx').on(table.chapterRevisionId, table.questionKey),
    check('chapter_questions_normalization_check', sql`${table.normalization} in ('exact', 'trim', 'trim_casefold')`),
    check(
      'chapter_questions_answer_hmac_len_check',
      sql`${table.answerHmac} is null or octet_length(${table.answerHmac}) = 32`
    ),
    check('chapter_questions_position_check', sql`${table.position} > 0`),
    check('chapter_questions_key_version_check', sql`${table.answerKeyVersion} > 0`),
  ]
);

export const chapterRevisionAssets = pgTable(
  'chapter_revision_assets',
  {
    chapterRevisionId: uuid('chapter_revision_id')
      .notNull()
      .references(() => chapterRevisions.id, { onDelete: 'cascade' }),
    assetId: uuid('asset_id')
      .notNull()
      .references(() => assets.id, { onDelete: 'restrict' }),
  },
  (table) => [
    primaryKey({
      columns: [table.chapterRevisionId, table.assetId],
      name: 'chapter_revision_assets_pk',
    }),
    index('chapter_revision_assets_asset_idx').on(table.assetId),
  ]
);
