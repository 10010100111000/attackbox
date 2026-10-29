import {
  pgTable,
  uuid,
  varchar,
  integer,
  timestamp,
  primaryKey,
  unique,
  index,
  check
} from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { modules } from './content'
export const learningRoutes = pgTable(
  'learning_routes',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    title: varchar('title', { length: 160 }).notNull(),
    summary: varchar('summary', { length: 500 }).notNull().default(''),
    status: varchar('status', { length: 20 }).notNull().default('unpublished'),
    lockVersion: integer('lock_version').notNull().default(1),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow()
  },
  t => [
    check('learning_routes_title_check', sql`length(trim(${t.title}))>0`),
    check('learning_routes_status_check', sql`${t.status} in ('unpublished','published')`),
    check('learning_routes_lock_version_check', sql`${t.lockVersion}>0`)
  ]
)
export const learningRouteModules = pgTable(
  'learning_route_modules',
  {
    routeId: uuid('route_id')
      .notNull()
      .references(() => learningRoutes.id, { onDelete: 'cascade' }),
    moduleId: uuid('module_id')
      .notNull()
      .references(() => modules.id, { onDelete: 'restrict' }),
    position: integer('position').notNull()
  },
  t => [
    primaryKey({ columns: [t.routeId, t.moduleId] }),
    unique().on(t.routeId, t.position),
    index('learning_route_modules_module_idx').on(t.moduleId),
    check('learning_route_modules_position_check', sql`${t.position}>0`)
  ]
)
