import { pgTable, uuid, timestamp, primaryKey, index } from 'drizzle-orm/pg-core'
import { users } from './auth'
import { chapters, chapterQuestions } from './content'
export const chapterProgress = pgTable(
  'chapter_progress',
  {
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    chapterId: uuid('chapter_id')
      .notNull()
      .references(() => chapters.id, { onDelete: 'cascade' }),
    startedAt: timestamp('started_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow(),
    lastVisitedAt: timestamp('last_visited_at', { withTimezone: true, mode: 'date' })
      .notNull()
      .defaultNow(),
    completedAt: timestamp('completed_at', { withTimezone: true, mode: 'date' })
  },
  table => [
    primaryKey({ columns: [table.userId, table.chapterId] }),
    index('chapter_progress_user_visited_idx').on(table.userId, table.lastVisitedAt)
  ]
)
export const questionCompletions = pgTable(
  'question_completions',
  {
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    questionId: uuid('question_id')
      .notNull()
      .references(() => chapterQuestions.id, { onDelete: 'cascade' }),
    passedAt: timestamp('passed_at', { withTimezone: true, mode: 'date' }).notNull().defaultNow()
  },
  table => [primaryKey({ columns: [table.userId, table.questionId] })]
)
