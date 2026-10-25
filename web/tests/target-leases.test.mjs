import assert from 'node:assert/strict'
import test from 'node:test'
import {learningFixture} from './helpers/learning-fixture.mjs'
import {sandboxWorker} from './helpers/sandbox-worker.mjs'

test('target lease is immutable, expiry reconciles without a page, next start gets a new identity', {skip:!process.env.CONTENT_TEST_DATABASE_URL||!process.env.NODE_TEST_CLUSTER_SECRET,timeout:45000},async()=>{
 const f=await learningFixture();let worker,template,learner
 try{
  worker=await sandboxWorker(f.base,process.env.NODE_TEST_CLUSTER_SECRET,{dockerTargets:true})
  learner=await f.account();const mod=await f.createModule()
  template=(await f.request('/api/admin/target-templates',{actor:f.admin,method:'POST',body:{name:'Lease test',image:'nginx:1.27',description:'',sourceUrl:'',ports:[],accessInstructions:'',enabled:true}})).data
  await f.sql`update modules set default_target_template_id=${template.id} where id=${mod.module.id}`
  const context={kind:'chapter',moduleSlug:mod.module.slug,chapterSlug:'intro'}
  const request=async()=>{
   const response=await fetch(f.base+'/api/targets',{method:'POST',headers:{cookie:`attackbox_session=${learner.token}`,'content-type':'application/json'},body:JSON.stringify({...context,action:'start'})})
   assert.equal(response.status,200);return response.json()
  }
  const before=Date.now(),first=await request()
  assert.ok(Date.parse(first.expiresAt)>=before+3599000 && Date.parse(first.expiresAt)<=Date.now()+3601000)
  const retry=await request();assert.equal(retry.id,first.id);assert.equal(retry.expiresAt,first.expiresAt)
  assert.equal(worker.requests.find(r=>r.type==='target_start').payload.expires_at,first.expiresAt)
  await f.sql`update target_instances set expires_at=now()-interval '1 second' where id=${first.id}`
  // No inspect request or page visit: the Nitro reconciliation loop must stop it.
  const deadline=Date.now()+25000
  while(true){
   const [row]=await f.sql`select state from target_instances where id=${first.id}`
   if(row.state==='stopped')break
   assert.ok(Date.now()<deadline,'background expiry did not converge')
   await new Promise(resolve=>setTimeout(resolve,100))
  }
  assert.ok(worker.requests.some(r=>r.type==='target_stop' && r.payload.target_id===first.id))
  const second=await request();assert.notEqual(second.id,first.id);assert.ok(Date.parse(second.expiresAt)>Date.now())
 }finally{
  if(worker)await worker.close()
  if(learner)await f.sql`delete from target_instances where user_id=${learner.id}`
  await f.cleanup()
  if(template){const postgres=(await import('postgres')).default;const sql=postgres(process.env.CONTENT_TEST_DATABASE_URL);await sql`delete from target_templates where id=${template.id}`;await sql.end()}
 }
})
