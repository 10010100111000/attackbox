import { drizzle } from 'drizzle-orm/postgres-js'
import postgres from 'postgres'
import * as schema from './schema'

const connectionString = process.env.DATABASE_URL?.trim()
const missingConfiguration = () => new Error('DATABASE_URL is required')

// Auth prerendering needs no database. Runtime processes must be configured,
// and the build-only client must never fall back to PostgreSQL environment defaults.
if (!connectionString && !import.meta.prerender) throw missingConfiguration()
const options = { max: 10, idle_timeout: 20, connect_timeout: 10 }
// postgres-js supports a custom socket factory; refusing it prevents any
// implicit PGHOST/PGUSER connection during database-free prerendering.
const prerenderOptions = {
  ...options,
  socket: async () => {
    throw missingConfiguration()
  }
}
export const client = connectionString
  ? postgres(connectionString, options)
  : postgres(prerenderOptions)

export const db = drizzle(client, { schema })

export type DB = typeof db
