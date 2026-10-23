import test from 'node:test'
import assert from 'node:assert/strict'
import path from 'node:path'
import fs from 'node:fs/promises'
import os from 'node:os'
import crypto from 'node:crypto'
import postgres from 'postgres'
import { drizzle } from 'drizzle-orm/postgres-js'
import { migrate } from 'drizzle-orm/postgres-js/migrator'
const target = process.env.CONTENT_MIGRATION_TEST_DATABASE_URL

test('fresh baseline has direct content records, seeded categories and no revision tables', { skip: !target }, async () => {
  const url = new URL(target)
  assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname))
  assert.ok(url.pathname.endsWith('_test'))
  const client = postgres(target, { max: 1 })
  let temporary
  try {
    const folder = path.resolve(import.meta.dirname, '../server/db/migrations')
    const [present] = await client`SELECT to_regclass('public.modules') AS name`
    let legacyId
    if (!present.name) {
      temporary = await fs.mkdtemp(path.join(os.tmpdir(), 'attackbox-baseline-'))
      await fs.mkdir(path.join(temporary, 'meta'))
      const journal = JSON.parse(await fs.readFile(path.join(folder, 'meta/_journal.json'), 'utf8'))
      await fs.writeFile(path.join(temporary, 'meta/_journal.json'), JSON.stringify({ ...journal, entries: journal.entries.slice(0, 1) }))
      await fs.copyFile(path.join(folder, journal.entries[0].tag + '.sql'), path.join(temporary, journal.entries[0].tag + '.sql'))
      await migrate(drizzle(client), { migrationsFolder: temporary })
      const userId = crypto.randomUUID(); legacyId = crypto.randomUUID()
      await client`INSERT INTO users (id,email,display_name) VALUES (${userId},'upgrade@example.test','Upgrade')`
      await client`INSERT INTO modules (id,slug,title,created_by) VALUES (${legacyId},'upgrade-existing','Existing module',${userId})`
    }
    await migrate(drizzle(client), { migrationsFolder: folder })
    if (legacyId) {
      const [retained] = await client`SELECT title,learning_outcomes FROM modules WHERE id=${legacyId}`
      assert.equal(retained.title, 'Existing module'); assert.deepEqual(retained.learning_outcomes, [])
    }
    await migrate(drizzle(client), { migrationsFolder: folder })
    const tables = (await client`SELECT tablename FROM pg_tables WHERE schemaname='public'`).map(t => t.tablename)
    assert.ok(tables.includes('module_prerequisites'));
    assert.ok(tables.includes('modules')); assert.ok(tables.includes('chapters')); assert.ok(tables.includes('chapter_questions'))
    assert.ok(!tables.some(t => t.includes('revision')))
    assert.equal((await client`SELECT id FROM content_categories`).length, 6)
    const columns = (await client`SELECT column_name FROM information_schema.columns WHERE table_name='modules' AND table_schema='public'`).map(c => c.column_name)
    assert.ok(columns.includes('status')); assert.ok(columns.includes('lock_version'))
    assert.ok(!columns.includes('published_revision_id'))
    const chapterColumns = (await client`SELECT column_name FROM information_schema.columns WHERE table_name='chapters' AND table_schema='public'`).map(c => c.column_name)
    assert.ok(chapterColumns.includes('duration_minutes'));
    assert.ok(chapterColumns.includes('body_markdown')); assert.ok(chapterColumns.includes('position'))
  } finally { await client.end(); if (temporary) await fs.rm(temporary, { recursive: true, force: true }) }
})
