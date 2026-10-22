// web/server/scripts/apply-migrations.mjs
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import postgres from 'postgres';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const DATABASE_URL = process.env.DATABASE_URL || 'postgres://attackbox:attackbox_pwd_9a7b12@127.0.0.1:5432/attackbox_db';

console.log(`Applying migrations to: ${DATABASE_URL.replace(/:[^:@]+@/, ':****@')}`);

const sql = postgres(DATABASE_URL, { max: 1 });

async function main() {
  // 建立 migrations 记录表（如果不存在）
  await sql`
    CREATE TABLE IF NOT EXISTS "__drizzle_migrations_journal" (
      id serial PRIMARY KEY,
      tag varchar(255) NOT NULL UNIQUE,
      applied_at timestamp with time zone DEFAULT now() NOT NULL
    )
  `;

  // 如果 users 表已存在，说明 0000_fancy_selene 已应用
  const [usersTable] = await sql`
    SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'users'
  `;
  if (usersTable) {
    await sql`
      INSERT INTO "__drizzle_migrations_journal" (tag) VALUES ('0000_fancy_selene') ON CONFLICT DO NOTHING
    `;
  }

  const migrationsDir = path.resolve(__dirname, '../db/migrations');
  const files = fs.readdirSync(migrationsDir).filter(f => f.endsWith('.sql')).sort();

  for (const file of files) {
    const tag = file.replace(/\.sql$/, '');
    const [existing] = await sql`
      SELECT id FROM "__drizzle_migrations_journal" WHERE tag = ${tag}
    `;

    if (existing) {
      console.log(`[SKIP] Migration already applied: ${tag}`);
      continue;
    }

    console.log(`[APPLY] Applying migration: ${tag}...`);
    const filePath = path.join(migrationsDir, file);
    const content = fs.readFileSync(filePath, 'utf-8');
    const statements = content
      .split('--> statement-breakpoint')
      .map(s => s.trim())
      .filter(Boolean);

    await sql.begin(async (tx) => {
      for (const stmt of statements) {
        await tx.unsafe(stmt);
      }
      await tx`
        INSERT INTO "__drizzle_migrations_journal" (tag) VALUES (${tag})
      `;
    });

    console.log(`[SUCCESS] Applied migration: ${tag}`);
  }

  // 检查生产库中的表数量
  const tables = await sql`
    SELECT table_name 
    FROM information_schema.tables 
    WHERE table_schema = 'public'
    ORDER BY table_name ASC
  `;
  console.log(`Current tables in target database (${tables.length}): ${tables.map(t => t.table_name).join(', ')}`);

  await sql.end();
}

main().catch(err => {
  console.error('[FAILED] Migration failed:', err);
  process.exit(1);
});
