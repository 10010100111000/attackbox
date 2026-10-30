import {
  pgTable,
  uuid,
  varchar,
  text,
  timestamp,
  integer,
  boolean,
  index,
  primaryKey,
  check
} from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import type { AdPlacement } from '../../../shared/advertising'

export const advertisements = pgTable(
  'advertisements',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    name: varchar('name', { length: 120 }).notNull(),
    title: varchar('title', { length: 120 }).notNull(),
    description: varchar('description', { length: 300 }).notNull().default(''),
    imageUrl: text('image_url').notNull().default(''),
    href: text('href').notNull(),
    cta: varchar('cta', { length: 30 }).notNull().default(''),
    sponsor: varchar('sponsor', { length: 80 }).notNull().default(''),
    placements: text('placements').array().$type<AdPlacement[]>().notNull(),
    enabled: boolean('enabled').notNull().default(false),
    weight: integer('weight').notNull().default(1),
    startsAt: timestamp('starts_at', { withTimezone: true }),
    endsAt: timestamp('ends_at', { withTimezone: true }),
    lockVersion: integer('lock_version').notNull().default(1),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow()
  },
  t => [
    check('advertisements_weight_check', sql`${t.weight} between 1 and 100`),
    check('advertisements_version_check', sql`${t.lockVersion} > 0`),
    check(
      'advertisements_window_check',
      sql`${t.startsAt} is null or ${t.endsAt} is null or ${t.startsAt} < ${t.endsAt}`
    ),
    check(
      'advertisements_placements_check',
      sql`cardinality(${t.placements}) between 1 and 2 and ${t.placements} <@ array['learn_catalog','reference_footer']::text[]`
    )
  ]
)
export const analyticsVisits = pgTable(
  'analytics_visits',
  {
    id: uuid('id').primaryKey(),
    visitorKey: varchar('visitor_key', { length: 64 }).notNull(),
    path: varchar('path', { length: 500 }).notNull(),
    source: varchar('source', { length: 253 }).notNull(),
    medium: varchar('medium', { length: 80 }).notNull(),
    campaign: varchar('campaign', { length: 120 }).notNull(),
    channel: varchar('channel', { length: 16 }).notNull(),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    readAt: timestamp('read_at', { withTimezone: true })
  },
  t => [
    index('analytics_visits_created_idx').on(t.createdAt),
    index('analytics_visits_visitor_idx').on(t.visitorKey, t.createdAt)
  ]
)
export const adEvents = pgTable(
  'ad_events',
  {
    visitId: uuid('visit_id')
      .notNull()
      .references(() => analyticsVisits.id, { onDelete: 'cascade' }),
    // Event facts keep the campaign identity/name after a campaign is deleted.
    adId: uuid('ad_id').notNull(),
    adName: varchar('ad_name', { length: 120 }).notNull(),
    placement: varchar('placement', { length: 32 }).$type<AdPlacement>().notNull(),
    kind: varchar('kind', { length: 16 }).$type<'impression' | 'click'>().notNull(),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow()
  },
  t => [
    primaryKey({ columns: [t.visitId, t.adId, t.placement, t.kind] }),
    index('ad_events_ad_created_idx').on(t.adId, t.createdAt),
    check('ad_events_kind_check', sql`${t.kind} in ('impression','click')`)
  ]
)
export const analyticsBudgets = pgTable(
  'analytics_budgets',
  {
    key: varchar('key', { length: 64 }).primaryKey(),
    attempts: integer('attempts').notNull(),
    windowAt: timestamp('window_at', { withTimezone: true }).notNull().defaultNow()
  },
  t => [index('analytics_budgets_window_idx').on(t.windowAt)]
)
