import test from 'node:test'
import assert from 'node:assert/strict'
import {rm} from 'node:fs/promises'
import {join} from 'node:path'
import {learningFixture} from './helpers/learning-fixture.mjs'
import {sandboxWorker} from './helpers/sandbox-worker.mjs'
test('Preparation cancel, retry, force rebuild, active deployment retention and late response CAS',{skip:!process.env.CONTENT_TEST_DATABASE_URL||!process.env.NODE_TEST_CLUSTER_SECRET,timeout:60000},async()=>{
 const f=await learningFixture(),options={composeLabs:true,composePrepareHold:true};let worker,template,browser;const users=[],instances=[]
 try{
  worker=await sandboxWorker(f.base,process.env.NODE_TEST_CLUSTER_SECRET,options)
  const uploaded=await fetch(f.base+'/api/admin/lab-bundles',{method:'POST',headers:{cookie:`attackbox_session=${f.admin.token}`,'content-type':'application/octet-stream','x-lab-filename':'compose.yml'},body:'services: {web: {image: nginx:1.27}}'});assert.equal(uploaded.status,200);const bundle=(await uploaded.json()).data
  const body={name:'Preparation lifecycle',description:'',accessInstructions:'',bundleId:bundle.id,settings:{file:'compose.yml',entryService:'web',entryNetwork:'default',entryPort:80,prefixBits:28},variables:{},enabled:false}
  template=(await f.request('/api/admin/target-templates',{actor:f.admin,method:'POST',body})).data
  const path=`/api/admin/target-templates/${template.id}/preparations`
  const act=async(action,job,status=200)=>(await f.request(path,{actor:f.admin,method:'POST',body:{action,nodeId:'test-node',expectedAttemptId:job?.attemptId},status})).data
  let job=await act('prepare');assert.equal(job.state,'preparing')
  const cancelled=await act('cancel',job);assert.equal(cancelled.state,'cancelled');assert.equal(cancelled.deploymentId,null)
  job=await act('prepare',cancelled);assert.notEqual(job.attemptId,cancelled.attemptId)
  await act('cancel',cancelled,409)
  options.composePrepareHold=false;options.composePrepareFail=true;job=await act('status');assert.equal(job.state,'failed')
  const failed=job;job=await act('prepare',job);assert.notEqual(job.attemptId,failed.attemptId);options.composePrepareFail=false;job=await act('status');assert.equal(job.state,'ready')
  const originalDeployment=job.deploymentId
  template=(await f.request(`/api/admin/target-templates/${template.id}`,{actor:f.admin,method:'PUT',body:{...body,enabled:true,lockVersion:template.lockVersion}})).data
  const mod=await f.createModule();await f.sql`update modules set default_target_template_id=${template.id} where id=${mod.module.id}`
  const context={kind:'chapter',moduleSlug:mod.module.slug,chapterSlug:'intro'}
  const start=async()=>{const u=await f.account();users.push(u);const r=await fetch(f.base+'/api/targets',{method:'POST',headers:{cookie:`attackbox_session=${u.token}`,'content-type':'application/json'},body:JSON.stringify({...context,action:'start'})});assert.equal(r.status,200);const result=await r.json();instances.push({u,id:result.id});return result}
  await start()
  options.composePrepareHold=true;job=await act('rebuild',job);assert.equal(job.deploymentId,originalDeployment);assert.equal(job.mode,'refresh');assert.equal(worker.requests.findLast(r=>r.type==='compose_prepare').payload.rebuild,true)
  await start();assert.equal(worker.requests.findLast(r=>r.type==='compose_start').payload.deployment_id,originalDeployment)
  await f.request(`/api/admin/target-templates/${template.id}`,{actor:f.admin,method:'PUT',body:{...body,settings:{...body.settings,entryPort:81},lockVersion:template.lockVersion},status:409})
  let release,entered;const started=new Promise(r=>entered=r);options.composeStatusGate={promise:new Promise(r=>release=r),enter:entered}
  const late=act('status');await started;delete options.composeStatusGate
  job=await act('cancel',job);assert.equal(job.state,'cancelled');assert.equal(job.deploymentId,originalDeployment)
  const prior=job;job=await act('prepare',job);assert.equal(job.mode,'refresh');assert.notEqual(job.attemptId,prior.attemptId)
  options.composePrepareHold=false;job=await act('status');assert.equal(job.state,'ready');assert.notEqual(job.deploymentId,originalDeployment)
  release();const lateResult=await late;assert.equal(lateResult.attemptId,job.attemptId);assert.equal(lateResult.state,'ready')
  await start();assert.equal(worker.requests.findLast(r=>r.type==='compose_start').payload.deployment_id,job.deploymentId)
  assert.equal(worker.requests.filter(r=>r.type==='compose_stop').length,0)
  if(process.env.COMPOSE_BROWSER_TEST==='1'){
   const {chromium}=await import('playwright');browser=await chromium.launch({executablePath:'/usr/bin/chromium',args:['--no-sandbox']});const ctx=await browser.newContext({viewport:{width:1440,height:1000}});await ctx.addCookies([{name:'attackbox_session',value:f.admin.token,url:f.base}]);const page=await ctx.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto(f.base+`/admin/target-templates/${template.id}`);await page.locator('.node-row').getByText('已准备',{exact:true}).waitFor()
   options.composePrepareHold=true;page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'强制重建',exact:true}).click();await page.locator('.node-row').getByText('准备中',{exact:true}).waitFor();await page.getByText('现有部署可用',{exact:true}).waitFor();assert.equal(await page.getByRole('checkbox').isChecked(),true)
   await page.getByRole('button',{name:'取消准备',exact:true}).click();await page.locator('.node-row').getByText('已取消',{exact:true}).waitFor();assert.deepEqual(errors,[]);await page.evaluate(()=>window.scrollTo(0,0));await page.screenshot({path:'/tmp/attackbox-preparation-management.png',fullPage:true})
  }
  for(const {u,id}of instances){const r=await fetch(f.base+'/api/targets',{method:'POST',headers:{cookie:`attackbox_session=${u.token}`,'content-type':'application/json'},body:JSON.stringify({action:'stop',id})});assert.equal(r.status,200)}
  const stops=worker.requests.filter(r=>r.type==='compose_stop');assert.equal(stops[0].payload.deployment_id,originalDeployment);assert.equal(stops[2].payload.deployment_id,job.deploymentId)
 }finally{
  await browser?.close();await worker?.close();for(const u of users)await f.sql`delete from target_instances where user_id=${u.id}`
  await f.sql`update modules set default_target_template_id=null where default_target_template_id=${template?.id||'00000000-0000-4000-8000-000000000000'}`
  if(template)await f.sql`delete from target_templates where id=${template.id}`
  const bundles=await f.sql`select id from lab_bundles where actor_id=${f.admin.id}`;await f.sql`delete from lab_bundles where actor_id=${f.admin.id}`;for(const {id}of bundles)if(process.env.LAB_DATA_DIR)await rm(join(process.env.LAB_DATA_DIR,id),{recursive:true,force:true});await f.cleanup()
 }
})
