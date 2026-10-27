import test from 'node:test'
import assert from 'node:assert/strict'
import {mkdtemp,mkdir,readFile,writeFile,copyFile,rm} from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import postgres from 'postgres'
import {drizzle} from 'drizzle-orm/postgres-js'
import {migrate} from 'drizzle-orm/postgres-js/migrator'
const target=process.env.CATEGORY_MIGRATION_TEST_DATABASE_URL

test('seed cleanup deletes untouched unused categories and preserves referenced, edited and authored categories',{skip:!target},async()=>{
 const url=new URL(target);assert.ok(['localhost','127.0.0.1','[::1]'].includes(url.hostname)&&url.pathname.endsWith('_test'))
 const client=postgres(target,{max:1}),dir=await mkdtemp(path.join(os.tmpdir(),'category-migration-'))
 try{
  const folder=path.resolve(import.meta.dirname,'../server/db/migrations')
  const journal=JSON.parse(await readFile(path.join(folder,'meta/_journal.json'),'utf8'))
  const old={...journal,entries:journal.entries.filter(e=>e.idx<4)}
  await mkdir(path.join(dir,'meta'));await writeFile(path.join(dir,'meta/_journal.json'),JSON.stringify(old))
  for(const entry of old.entries)await copyFile(path.join(folder,entry.tag+'.sql'),path.join(dir,entry.tag+'.sql'))
  await migrate(drizzle(client),{migrationsFolder:dir})
  assert.equal((await client`select id from content_categories`).length,6)
  const [user]=await client`insert into users (email,display_name) values ('category-migration@example.test','Migration') returning id`
  const [category]=await client`select id from content_categories where slug='web-security'`
  await client`insert into modules(slug,title,created_by,category_id) values ('referenced-module','Referenced',${user.id},${category.id})`
  await client`update content_categories set name='Edited Linux' where slug='linux'`
  await client`insert into content_categories (slug,name) values ('custom-category','Authored category')`
  await migrate(drizzle(client),{migrationsFolder:folder})
  const rows=await client`select slug,name from content_categories order by slug`
  assert.deepEqual(rows.map(r=>r.slug),['custom-category','linux','web-security'])
  assert.equal(rows.find(r=>r.slug==='linux').name,'Edited Linux')
  assert.equal((await client`select category_id from modules where slug='referenced-module'`)[0].category_id,category.id)
  await migrate(drizzle(client),{migrationsFolder:folder})
  assert.equal((await client`select id from content_categories`).length,3)
 }finally{await client.end();await rm(dir,{recursive:true,force:true})}
})
