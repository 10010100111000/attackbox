import postgres from 'postgres'
if (!process.env.DATABASE_URL) throw new Error('DATABASE_URL is required')
const database = decodeURIComponent(new URL(process.env.DATABASE_URL).pathname.slice(1))
if (!database || process.env.RESET_DATABASE !== database) throw new Error('Set RESET_DATABASE to the target database name to erase its data.')
const client = postgres(process.env.DATABASE_URL, { max: 1 })
try {
  await client.begin(async tx => {
    await tx.unsafe('DROP SCHEMA IF EXISTS public CASCADE')
    await tx.unsafe('DROP SCHEMA IF EXISTS drizzle CASCADE')
    await tx.unsafe('CREATE SCHEMA public')
  })
} finally { await client.end() }
await import('./apply-migrations.mjs')
