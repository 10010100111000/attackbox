import { pgTable, integer, text, uuid, timestamp, uniqueIndex } from 'drizzle-orm/pg-core'
import { modules, chapters, chapterQuestions } from './content'
export const contentRepository = pgTable('content_repository', {
  id: integer('id').primaryKey(),
  repository: text('repository').notNull(),
  branch: text('branch').notNull(),
  directory: text('directory').notNull(),
  tokenCipher: text('token_cipher').notNull(),
  version: integer('version').notNull().default(1),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow()
})
export const moduleSources = pgTable(
  'module_sources',
  {
    moduleId: uuid('module_id')
      .primaryKey()
      .references(() => modules.id, { onDelete: 'cascade' }),
    origin: text('origin').notNull(),
    sourcePath: text('source_path').notNull(),
    sourceHash: text('source_hash').notNull(),
    commit: text('commit'),
    syncedAt: timestamp('synced_at', { withTimezone: true }).notNull().defaultNow()
  },
  t => [uniqueIndex('module_sources_origin_path_unique').on(t.origin, t.sourcePath)]
)
export const chapterSources = pgTable(
  'chapter_sources',
  {
    chapterId: uuid('chapter_id')
      .primaryKey()
      .references(() => chapters.id, { onDelete: 'cascade' }),
    moduleId: uuid('module_id')
      .notNull()
      .references(() => modules.id, { onDelete: 'cascade' }),
    matchKey: text('match_key').notNull()
  },
  t => [uniqueIndex('chapter_sources_module_key_unique').on(t.moduleId, t.matchKey)]
)
export const questionSources = pgTable(
  'question_sources',
  {
    questionId: uuid('question_id')
      .primaryKey()
      .references(() => chapterQuestions.id, { onDelete: 'cascade' }),
    chapterId: uuid('chapter_id')
      .notNull()
      .references(() => chapters.id, { onDelete: 'cascade' }),
    matchKey: text('match_key').notNull()
  },
  t => [uniqueIndex('question_sources_chapter_key_unique').on(t.chapterId, t.matchKey)]
)
