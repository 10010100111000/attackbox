import {
  pgTable,
  uuid,
  varchar,
  text,
  boolean,
  jsonb,
  integer,
  timestamp,
  check
} from 'drizzle-orm/pg-core'
import { sql } from 'drizzle-orm'
import { labBundles } from './lab-bundles'
import type { ComposeSettings } from '../../../shared/compose-labs'
export const targetTemplates = pgTable(
  'target_templates',
  {
    revision: integer('revision').notNull().default(1),
    bundleId: uuid('bundle_id')
      .notNull()
      .references(() => labBundles.id, { onDelete: 'restrict' }),
    settings: jsonb('settings').$type<ComposeSettings>().notNull(),
    variablesCipher: text('variables_cipher'),
    variableKeys: jsonb('variable_keys').$type<string[]>().notNull().default([]),
    id: uuid('id').primaryKey().defaultRandom(),
    name: varchar('name', { length: 120 }).notNull(),
    description: varchar('description', { length: 500 }).notNull().default(''),
    accessInstructions: text('access_instructions').notNull().default(''),
    enabled: boolean('enabled').notNull().default(false),
    lockVersion: integer('lock_version').notNull().default(1),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow()
  },
  t => [
    check('target_templates_name_check', sql`length(trim(${t.name}))>0`),
    check('target_templates_lock_check', sql`${t.lockVersion}>0`),
    check('target_templates_revision_check', sql`${t.revision}>0`),
    check('target_templates_settings_check', sql`jsonb_typeof(${t.settings})='object'`)
  ]
)
