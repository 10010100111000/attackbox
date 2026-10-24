import test from 'node:test'
import assert from 'node:assert/strict'
import {mkdtemp,readFile,writeFile,copyFile,mkdir,rm} from 'node:fs/promises'
import {tmpdir} from 'node:os'
import path from 'node:path'
import postgres from 'postgres'
import {drizzle} from 'drizzle-orm/postgres-js'
import {migrate} from 'drizzle-orm/postgres-js/migrator'
test('route migration upgrades the prior baseline without deleting existing accounts or modules',{skip:!process.env.ROUTES_MIGRATION_TEST_DATABASE_URL},async()=>{
 const url=new URL(process.env.ROUTES_MIGRATION_TEST_DATABASE_URL)
 assert.ok(['localhost','127.0.0.1'].includes(url.hostname)&&url.pathname.endsWith('_test'))
 const sql=postgres(url.toString()),folder=await mkdtemp(path.join(tmpdir(),'route-migration-')),source=path.resolve(import.meta.dirname,'../server/db/migrations')
 try{
  const [before]=await sql`select to_regclass('public.users') as users`;assert.equal(before.users,null,'use a fresh dedicated migration database')
  const journal=JSON.parse(await readFile(path.join(source,'meta/_journal.json'),'utf8'))
  await mkdir(path.join(folder,'meta'))
  const old={...journal,entries:journal.entries.filter(x=>x.idx<5)}
  await writeFile(path.join(folder,'meta/_journal.json'),JSON.stringify(old))
  for(const e of old.entries)await copyFile(path.join(source,e.tag+'.sql'),path.join(folder,e.tag+'.sql'))
  await migrate(drizzle(sql),{migrationsFolder:folder})
  const [u]=await sql`insert into users(email,display_name,role,status) values('upgrade@example.test','Upgrade','admin','active') returning id`
  const [m]=await sql`insert into modules(slug,title,created_by) values('upgrade-content','Existing content',${u.id}) returning id`
  await migrate(drizzle(sql),{migrationsFolder:source})
  assert.equal((await sql`select title from modules where id=${m.id}`)[0].title,'Existing content')
  assert.equal((await sql`select count(*)::int as count from users where id=${u.id}`)[0].count,1)
  const [r]=await sql`insert into learning_routes(title) values('Upgraded route') returning id`
  await sql`insert into learning_route_modules(route_id,module_id,position) values(${r.id},${m.id},1)`
  assert.equal((await sql`select count(*)::int as count from learning_route_modules`)[0].count,1)
 }finally{await sql.end();await rm(folder,{recursive:true,force:true})}
})
