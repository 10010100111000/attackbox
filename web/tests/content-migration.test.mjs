import test from 'node:test'
import assert from 'node:assert/strict'
import path from 'node:path'
import postgres from 'postgres'
import { drizzle } from 'drizzle-orm/postgres-js'
import { migrate } from 'drizzle-orm/postgres-js/migrator'
const target = process.env.CONTENT_MIGRATION_TEST_DATABASE_URL

test('fresh baseline has direct content records, seeded categories and no revision tables', { skip: !target }, async () => {
  const url = new URL(target)
  assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname))
  assert.ok(url.pathname.endsWith('_test'))
  const client = postgres(target, { max: 1 })
  try {
    const folder = path.resolve(import.meta.dirname, '../server/db/migrations')
    await migrate(drizzle(client), { migrationsFolder: folder })
    // 验证重复执行幂等性
    await migrate(drizzle(client), { migrationsFolder: folder })
    const tables = (await client`SELECT tablename FROM pg_tables WHERE schemaname='public'`).map(t => t.tablename)
    assert.ok(tables.includes('module_prerequisites'))
    assert.ok(tables.includes('modules'))
    assert.ok(tables.includes('chapters'))
    assert.ok(tables.includes('chapter_questions'))
    assert.ok(tables.includes('chapter_progress'))
    assert.ok(tables.includes('question_completions'))
    assert.ok(tables.includes('sandbox_identities'))
    assert.ok(!tables.some(t => t.includes('revision')))
    assert.equal((await client`SELECT id FROM content_categories`).length, 6)
    const columns = (await client`SELECT column_name FROM information_schema.columns WHERE table_name='modules' AND table_schema='public'`).map(c => c.column_name)
    assert.ok(columns.includes('status'))
    assert.ok(columns.includes('lock_version'))
    assert.ok(columns.includes('learning_outcomes'))
    assert.ok(!columns.includes('published_revision_id'))
    const chapterColumns = (await client`SELECT column_name FROM information_schema.columns WHERE table_name='chapters' AND table_schema='public'`).map(c => c.column_name)
    assert.ok(chapterColumns.includes('duration_minutes'))
    assert.ok(chapterColumns.includes('body_markdown'))
    assert.ok(chapterColumns.includes('position'))
    const questionColumns = (await client`SELECT column_name FROM information_schema.columns WHERE table_name='chapter_questions' AND table_schema='public'`).map(c => c.column_name)
    assert.ok(questionColumns.includes('answer'))
  } finally { await client.end() }
})
