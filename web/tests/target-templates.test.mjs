import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import {learningFixture} from './helpers/learning-fixture.mjs'
test('Retired container APIs fail explicitly and legacy records cannot become released',{skip:!process.env.CONTENT_TEST_DATABASE_URL},async()=>{
 const f=await learningFixture();let template,user
 try{
  user=await f.account();const id=crypto.randomUUID();template=id
  await f.sql`insert into target_templates(id,name,image,runtime) values(${id},'Retired fixture','nginx:1.27','container')`
  await f.request('/api/admin/target-templates',{actor:f.admin,method:'POST',body:{},status:410})
  await f.request(`/api/admin/target-templates/${id}`,{actor:f.admin,method:'PUT',body:{},status:410})
  await f.request(`/api/admin/target-templates/${id}/image`,{actor:f.admin,method:'POST',body:{action:'prepare'},status:410})
  await f.request(`/api/admin/target-templates/${id}`,{actor:f.admin,status:410})
  const list=(await f.request('/api/admin/target-templates',{actor:f.admin})).data;assert.ok(!list.items.some(t=>t.id===id))
  const instance='tgt_'+crypto.randomBytes(16).toString('hex')
  await f.sql`insert into target_instances(id,user_id,scope,node_id,boot_id,state,image,ports,runtime,expires_at) values(${instance},${user.id},'legacy','offline','old','running','nginx:1.27','[]','container',now()+interval '1 hour')`
  const result=await fetch(f.base+'/api/targets',{method:'POST',headers:{cookie:`attackbox_session=${user.token}`,'content-type':'application/json'},body:JSON.stringify({action:'stop',id:instance})});assert.equal(result.status,410)
  assert.equal((await f.sql`select state from target_instances where id=${instance}`)[0].state,'running')
 }finally{if(user)await f.sql`delete from target_instances where user_id=${user.id}`;if(template)await f.sql`delete from target_templates where id=${template}`;await f.cleanup()}
})
