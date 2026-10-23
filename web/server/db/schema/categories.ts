import { pgTable, uuid, varchar, integer, uniqueIndex, check } from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
export const contentCategories = pgTable('content_categories', {
  id: uuid('id').primaryKey().defaultRandom(),
  slug: varchar('slug', { length: 100 }).notNull(),
  name: varchar('name', { length: 80 }).notNull(),
  position: integer('position').notNull().default(0),
}, table => [uniqueIndex('content_categories_slug_unique').on(table.slug),
  uniqueIndex('content_categories_name_unique').on(table.name),
  check('content_categories_slug_check', sql`${table.slug} ~ '^[a-z0-9]+(-[a-z0-9]+)*$'`)])
