import test from 'node:test'
import assert from 'node:assert/strict'
import {execFileSync} from 'node:child_process'
import postgres from 'postgres'
const database=process.env.CONTENT_MIGRATION_TEST_DATABASE_URL
test('Fresh initializer is idempotent and rejects a mismatched journal without modifying data',{skip:!database},async()=>{
 const url=new URL(database);assert.ok(['localhost','127.0.0.1'].includes(url.hostname)&&url.pathname.endsWith('_test'))
 const client=postgres(database,{max:1}),run=()=>execFileSync(process.execPath,['web/server/scripts/apply-migrations.mjs'],{cwd:new URL('../../',import.meta.url),env:{...process.env,DATABASE_URL:database},stdio:'pipe'})
 let hash
 try{
  run();run();const rows=await client`select hash from drizzle.__drizzle_migrations`;assert.equal(rows.length,1);hash=rows[0].hash
  assert.equal((await client`select count(*)::int as n from content_categories`)[0].n,6)
  await client`update drizzle.__drizzle_migrations set hash='different-baseline'`
  assert.throws(run,e=>String(e.stderr).includes('does not match this fresh-install baseline'))
  assert.equal((await client`select count(*)::int as n from content_categories`)[0].n,6)
 }finally{if(hash)await client`update drizzle.__drizzle_migrations set hash=${hash}`;await client.end()}
})
