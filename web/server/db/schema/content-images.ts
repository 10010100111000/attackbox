import { pgTable, uuid, text, boolean, primaryKey } from 'drizzle-orm/pg-core'
import { modules } from './content'
import { assets } from './assets'
export const moduleImages = pgTable(
  'module_images',
  {
    moduleId: uuid('module_id')
      .notNull()
      .references(() => modules.id, { onDelete: 'cascade' }),
    href: text('href').notNull(),
    assetId: uuid('asset_id')
      .notNull()
      .references(() => assets.id, { onDelete: 'restrict' }),
    public: boolean('public').notNull().default(false)
  },
  t => [primaryKey({ columns: [t.moduleId, t.href] })]
)
