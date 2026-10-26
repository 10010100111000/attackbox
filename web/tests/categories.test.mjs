import test from 'node:test'
import assert from 'node:assert/strict'
import {mkdtemp,writeFile,rm} from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import {createJiti} from 'jiti'
import {learningFixture} from './helpers/learning-fixture.mjs'

test('Git-driven category lifecycle: preset matching, auto-creation, orphan pruning and retired admin endpoints',{skip:!process.env.CONTENT_TEST_DATABASE_URL||!process.env.CONTENT_TEST_BASE_URL},async()=>{
 process.env.DATABASE_URL=process.env.CONTENT_TEST_DATABASE_URL
 const f=await learningFixture(),jiti=createJiti(import.meta.url)
 const sync=await jiti.import('../server/services/content-sync.ts'),database=await jiti.import('../server/db/index.ts')
 const root=await mkdtemp(path.join(os.tmpdir(),'category-git-test-')),moduleIds=new Set()
 const customCategory='移动安全 '+crypto.randomUUID().slice(0,8)
 try{
  const dead=await fetch(f.base+'/api/admin/categories',{method:'POST',headers:{cookie:`attackbox_session=${f.admin.token}`,'content-type':'application/json'},body:JSON.stringify({name:'retired'})})
  assert.ok([404,405].includes(dead.status),'manual VPS category endpoint is retired')
  await writeFile(path.join(root,'CategoryModule.md'),`---\ncategory: ${customCategory}\n---\n\n# 第一章\n\n正文`)
  let preview=await sync.createLocalContentPreview(root,f.admin.id)
  assert.equal(preview.conflicts.length,0)
  const applied=await sync.applyContentPreview(preview.id,f.admin.id)
  const id=applied.moduleIds[0];moduleIds.add(id)
  let detail=(await f.request(`/api/admin/modules/${id}`,{actor:f.admin})).data
  assert.equal(detail.category.name,customCategory)
  assert.ok((await f.request('/api/categories')).data.some(c=>c.name===customCategory))
  // Switching to a preset category slug maps to the preset category and prunes the unreferenced custom category.
  await writeFile(path.join(root,'CategoryModule.md'),'---\ncategory: web-security\n---\n\n# 第一章\n\n正文')
  preview=await sync.createLocalContentPreview(root,f.admin.id)
  await sync.applyContentPreview(preview.id,f.admin.id)
  detail=(await f.request(`/api/admin/modules/${id}`,{actor:f.admin})).data
  assert.equal(detail.category.slug,'web-security')
  assert.equal(detail.category.name,'Web 安全')
  const categoriesAfter=(await f.request('/api/categories')).data
  assert.ok(!categoriesAfter.some(c=>c.name===customCategory),'orphan custom category is automatically pruned')
  assert.ok(categoriesAfter.some(c=>c.slug==='linux'),'preset categories remain intact')
 }finally{
  await f.sql`delete from content_previews where actor_id=${f.admin.id}`
  for(const id of moduleIds){await f.sql`delete from audit_logs where target_id=${id}`;await f.sql`delete from modules where id=${id}`}
  await f.sql`delete from content_categories where name=${customCategory}`
  await f.cleanup();await database.client.end();await rm(root,{recursive:true,force:true})
 }
})
