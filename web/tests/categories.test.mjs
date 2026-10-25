import test from 'node:test'
import assert from 'node:assert/strict'
import {learningFixture} from './helpers/learning-fixture.mjs'
test('admin category lifecycle and reference protection', {skip:!process.env.CONTENT_TEST_DATABASE_URL},async()=>{
 const f=await learningFixture();let category
 try{
  await f.request('/api/admin/categories',{status:401})
  const learner=await f.account('user')
  await f.request('/api/admin/categories',{actor:learner,status:403})
  const name='Category '+crypto.randomUUID()
  category=(await f.request('/api/admin/categories',{actor:f.admin,method:'POST',body:{name}})).data
  const renamed=(await f.request(`/api/admin/categories/${category.id}`,{actor:f.admin,method:'PATCH',body:{currentName:name,name:name+' renamed'}})).data
  await f.request(`/api/admin/categories/${category.id}`,{actor:f.admin,method:'PATCH',body:{currentName:name,name:'Stale name'},status:409})
  const module=await f.createModule()
  await f.sql`update modules set category_id=${category.id} where id=${module.module.id}`
  await f.request(`/api/admin/categories/${category.id}`,{actor:f.admin,method:'DELETE',body:{currentName:renamed.name,moduleCount:0},status:409})
  const list=(await f.request('/api/admin/categories',{actor:f.admin})).data
  assert.equal(list.find(c=>c.id===category.id).moduleCount,1)
  await f.request(`/api/admin/categories/${category.id}`,{actor:f.admin,method:'DELETE',body:{currentName:renamed.name,moduleCount:1}})
  const [changed]=await f.sql`select category_id,status from modules where id=${module.module.id}`
  assert.equal(changed.category_id,null);assert.equal(changed.status,'published')
  const publicList=(await f.request('/api/categories')).data
  assert.ok(!publicList.some(c=>c.id===category.id))
 }finally{
  if(category) {await f.sql`update modules set category_id=null where category_id=${category.id}`;await f.sql`delete from content_categories where id=${category.id}`}
  await f.cleanup()
 }
})
