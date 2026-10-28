import test from 'node:test'
import assert from 'node:assert/strict'
import {execFileSync} from 'node:child_process'
import {readMigrationFiles} from 'drizzle-orm/migrator'
import postgres from 'postgres'
const database=process.env.CONTENT_MIGRATION_TEST_DATABASE_URL
test('Fresh initializer is idempotent and rejects a mismatched journal without modifying data',{skip:!database},async()=>{
 const url=new URL(database);assert.ok(['localhost','127.0.0.1'].includes(url.hostname)&&url.pathname.endsWith('_test'))
 const client=postgres(database,{max:1}),run=()=>execFileSync(process.execPath,['web/server/scripts/apply-migrations.mjs'],{cwd:new URL('../../',import.meta.url),env:{...process.env,DATABASE_URL:database},stdio:'pipe'})
 let applied
 try{
  run();run();applied=await client`select id,hash from drizzle.__drizzle_migrations order by created_at,id`;assert.equal(applied.length,readMigrationFiles({migrationsFolder:new URL('../server/db/migrations',import.meta.url).pathname}).length)
  assert.equal((await client`select count(*)::int as n from content_categories`)[0].n,0)
  await client`update drizzle.__drizzle_migrations set hash='different-baseline'`
  assert.throws(run,e=>String(e.stderr).includes('does not match this fresh-install baseline'))
  assert.equal((await client`select count(*)::int as n from content_categories`)[0].n,0)
 }finally{if(applied)for(const row of applied)await client`update drizzle.__drizzle_migrations set hash=${row.hash} where id=${row.id}`;await client.end()}
})
