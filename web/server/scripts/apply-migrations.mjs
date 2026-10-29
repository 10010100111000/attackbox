import path from 'node:path'
import postgres from 'postgres'
import { drizzle } from 'drizzle-orm/postgres-js'
import { migrate } from 'drizzle-orm/postgres-js/migrator'
import { readMigrationFiles } from 'drizzle-orm/migrator'

if (!process.env.DATABASE_URL) throw Error('DATABASE_URL is required')
const client = postgres(process.env.DATABASE_URL, { max: 1 })
const config = { migrationsFolder: path.resolve(import.meta.dirname, '../db/migrations') }
try {
  await client`select pg_advisory_lock(72486213)`
  const [journal] = await client`select to_regclass('drizzle.__drizzle_migrations') as name`
  const expected = readMigrationFiles(config)
  if (journal.name) {
    const applied =
      await client`select hash from drizzle.__drizzle_migrations order by created_at,id`
    if (applied.length > expected.length || applied.some((row, i) => row.hash !== expected[i].hash))
      throw Error(
        'Database does not match this fresh-install baseline. Use a new empty database; automatic conversion is not supported.'
      )
  } else {
    const tables = await client`select tablename from pg_tables where schemaname='public' limit 1`
    if (tables.length)
      throw Error(
        'An empty database is required for this baseline. No existing tables were changed.'
      )
  }
  await migrate(drizzle(client), config)
  console.log('Database schema initialized successfully.')
} finally {
  await client.end()
}
