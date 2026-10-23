import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import crypto from 'node:crypto'
import postgres from 'postgres'
import { drizzle } from 'drizzle-orm/postgres-js'
import { migrate } from 'drizzle-orm/postgres-js/migrator'
const target=process.env.CONTENT_MIGRATION_TEST_DATABASE_URL

test('category migration upgrades existing published content without recategorizing or losing it', {skip:!target}, async()=>{
  const url=new URL(target)
  assert.ok(['127.0.0.1','localhost','[::1]'].includes(url.hostname))
  assert.ok(url.pathname.endsWith('_test'))
  const sql=postgres(target,{max:1}),temporary=await fs.mkdtemp(path.join(os.tmpdir(),'attackbox-old-migrations-'))
  try{
    const [present]=await sql`SELECT to_regclass('public.users') AS table_name`
    assert.equal(present.table_name,null,'migration test requires a fresh isolated database')
    const folder=path.resolve(import.meta.dirname,'../server/db/migrations')
    const journal=JSON.parse(await fs.readFile(path.join(folder,'meta/_journal.json'),'utf8'))
    await fs.mkdir(path.join(temporary,'meta'))
    await fs.writeFile(path.join(temporary,'meta/_journal.json'),JSON.stringify({...journal,entries:journal.entries.slice(0,2)}))
    for(const entry of journal.entries.slice(0,2))await fs.copyFile(path.join(folder,`${entry.tag}.sql`),path.join(temporary,`${entry.tag}.sql`))
    await migrate(drizzle(sql),{migrationsFolder:temporary})
    const user=crypto.randomUUID(),module=crypto.randomUUID(),revision=crypto.randomUUID()
    await sql`INSERT INTO users (id,email,display_name) VALUES (${user},'migration@example.test','Migration Test')`
    await sql`INSERT INTO modules (id,slug,created_by) VALUES (${module},'legacy-module',${user})`
    await sql`INSERT INTO module_revisions (id,module_id,revision_no,state,title,difficulty,created_by) VALUES (${revision},${module},1,'published','Legacy published title','advanced',${user})`
    await sql`UPDATE modules SET published_revision_id=${revision} WHERE id=${module}`
    await migrate(drizzle(sql),{migrationsFolder:folder})
    const [after]=await sql`SELECT m.published_revision_id,r.title,r.difficulty,r.category_id FROM modules m JOIN module_revisions r ON r.id=m.published_revision_id WHERE m.id=${module}`
    assert.equal(after.published_revision_id,revision);assert.equal(after.title,'Legacy published title')
    assert.equal(after.difficulty,'advanced');assert.equal(after.category_id,null)
    const categories=await sql`SELECT slug FROM content_categories ORDER BY position`
    assert.equal(categories.length,6)
    await migrate(drizzle(sql),{migrationsFolder:folder})
    assert.equal((await sql`SELECT id FROM content_categories`).length,6,'migration replay is a no-op')
  }finally{await sql.end();await fs.rm(temporary,{recursive:true,force:true})}
})
