import path from 'node:path'
import postgres from 'postgres'
import { drizzle } from 'drizzle-orm/postgres-js'
import { migrate } from 'drizzle-orm/postgres-js/migrator'

if (!process.env.DATABASE_URL) throw new Error('DATABASE_URL is required')
const client = postgres(process.env.DATABASE_URL, { max: 1 })
try {
  const [legacy] = await client`SELECT to_regclass('public.module_revisions') AS old_content,
    to_regclass('public.__drizzle_migrations_journal') AS old_journal`
  if (legacy.old_content || legacy.old_journal) throw new Error('This baseline requires a fresh database. Reset the old database before starting this release.')
  await migrate(drizzle(client), { migrationsFolder: path.resolve(import.meta.dirname, '../db/migrations') })
  console.log('Database schema initialized successfully.')
} finally { await client.end() }
