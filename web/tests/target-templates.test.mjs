import test from 'node:test'
import assert from 'node:assert/strict'
import {learningFixture} from './helpers/learning-fixture.mjs'
test('Fresh Compose schema has one template contract, required references and no legacy columns',{skip:!process.env.CONTENT_TEST_DATABASE_URL},async()=>{
 const f=await learningFixture();let template
 try{
  template=await f.seedTarget({name:'Canonical template'})
  for(const table of ['target_templates','target_instances']){
   const columns=await f.sql`select column_name,is_nullable from information_schema.columns where table_schema='public' and table_name=${table}`
   for(const removed of ['runtime','image','ports','source_url','compose_revision','compose_settings'])assert.ok(!columns.some(c=>c.column_name===removed),`${table}.${removed} remains`)
   for(const field of table==='target_templates'?['bundle_id','settings','revision']:['deployment_id'])assert.equal(columns.find(c=>c.column_name===field)?.is_nullable,'NO')
  }
  const record=(await f.request(`/api/admin/target-templates/${template.id}`,{actor:f.admin})).data
  assert.equal(record.settings.entryPort,80);assert.equal(record.revision,1);assert.ok(!('runtime'in record))
  assert.equal((await fetch(f.base+'/api/admin/compose-templates',{method:'POST',headers:{cookie:`attackbox_session=${f.admin.token}`,'content-type':'application/json'},body:'{}'})).status,404)
  assert.equal((await fetch(f.base+`/api/admin/target-templates/${template.id}/image`,{method:'POST',headers:{cookie:`attackbox_session=${f.admin.token}`,'content-type':'application/json'},body:'{}'})).status,404)
  await assert.rejects(f.sql`delete from lab_bundles where id=${template.bundleId}`,e=>e.code==='23503')
 }finally{await f.cleanup()}
})
