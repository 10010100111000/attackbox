import assert from 'node:assert/strict'
import test from 'node:test'
import {rm} from 'node:fs/promises'
import {join} from 'node:path'
import {learningFixture} from './helpers/learning-fixture.mjs'
import {sandboxWorker} from './helpers/sandbox-worker.mjs'
import {labZip} from './helpers/lab-zip.mjs'

test('Compose upload, private delivery, manual preparation, permissions and learner lifecycle',{skip:!process.env.CONTENT_TEST_DATABASE_URL||!process.env.NODE_TEST_CLUSTER_SECRET,timeout:60000},async()=>{
 const f=await learningFixture();let worker,template,mod,learner,other;const bundles=[],extraTemplates=[]
 const send=async(path,{actor=f.admin,method='GET',body,raw,headers={}}={})=>{const res=await fetch(f.base+path,{method,headers:{...(actor?{cookie:`attackbox_session=${actor.token}`} :{}),...(body?{'content-type':'application/json'}:{}),...headers},body:raw|| (body?JSON.stringify(body):undefined)});const text=await res.text();return {res,json:text?JSON.parse(text):null}}
 try{
  worker=await sandboxWorker(f.base,process.env.NODE_TEST_CLUSTER_SECRET,{composeLabs:true});learner=await f.account();other=await f.account()
  const yaml="services:\n  web:\n    image: '${LAB_IMAGE}'\n    ports: ['127.0.0.1:18080:8080']\n    environment: {PASSWORD: '${PASSWORD:?required}'}\n    volumes: ['./init.sql:/init.sql:ro']\n"
  const data=labZip([{name:'lab/compose.yml',data:yaml},{name:'lab/init.sql',data:'select 1;'}])
  const upload=await send('/api/admin/lab-bundles',{method:'POST',raw:data,headers:{'content-type':'application/octet-stream','x-lab-filename':'lab.zip'}});assert.equal(upload.res.status,200);const bundle=upload.json.data;bundles.push(bundle.id);assert.deepEqual(bundle.missingFiles,[])
  assert.equal((await send(`/api/admin/lab-bundles/${bundle.id}`,{actor:learner})).res.status,403)
  assert.equal((await send(`/api/worker/lab-bundles/${bundle.id}`,{actor:null})).res.status,401)
  const credentials={'Authorization':`Bearer ${process.env.NODE_TEST_CLUSTER_SECRET}`,'X-Node-ID':'test-node'}
  const downloaded=await send(`/api/worker/lab-bundles/${bundle.id}`,{actor:null,headers:credentials});assert.equal(downloaded.res.status,200);assert.equal(downloaded.json.hash,bundle.hash);assert.ok(downloaded.json.files['lab/init.sql']);assert.match(downloaded.res.headers.get('cache-control'),/no-store/)
  const body={name:'Compose integration',description:'Private stack',accessInstructions:'Open /',bundleId:bundle.id,settings:{file:'lab/compose.yml',entryService:'web',entryNetwork:'default',entryPort:8080,prefixBits:28},variables:{LAB_IMAGE:'fixture:local',PASSWORD:'private-test-value'},enabled:false}
  assert.equal((await send('/api/admin/compose-templates',{method:'POST',body:{...body,enabled:true}})).res.status,409)
  const saved=await send('/api/admin/compose-templates',{method:'POST',body});assert.equal(saved.res.status,200);template=saved.json.data
  assert.equal(template.runtime,'compose');assert.equal(template.composeRevision,1);assert.ok(!JSON.stringify(saved.json).includes('private-test-value'));assert.ok(!('variablesCipher'in template))
  const [db]=await f.sql`select variables_cipher from target_templates where id=${template.id}`;assert.ok(db.variables_cipher&&!db.variables_cipher.includes('private-test-value'))
  assert.equal((await send('/api/admin/compose-templates',{actor:learner,method:'POST',body})).res.status,403)
  const prepPath=`/api/admin/compose-templates/${template.id}/preparations`
  const initial=await send(prepPath);assert.equal(initial.res.status,200);assert.equal(initial.json.data.nodes[0].available,true)
  const prep=await send(prepPath,{method:'POST',body:{nodeId:'test-node',action:'prepare'}});assert.equal(prep.res.status,200);assert.equal(prep.json.data.state,'preparing')
  const queried=await send(prepPath,{method:'POST',body:{nodeId:'test-node',action:'status'}});assert.equal(queried.json.data.state,'ready');assert.ok(queried.json.data.logs.length)
  const enabled=await send(`/api/admin/compose-templates/${template.id}`,{method:'PUT',body:{...body,variables:{},enabled:true,lockVersion:template.lockVersion}});assert.equal(enabled.res.status,200);template=enabled.json.data;assert.equal(template.composeRevision,1)
  const list=await send('/api/admin/target-templates');assert.ok(!JSON.stringify(list.json).includes('private-test-value'));assert.ok(!JSON.stringify(list.json).includes('variablesCipher'))
  mod=await f.createModule();await f.sql`update modules set default_target_template_id=${template.id} where id=${mod.module.id}`
  const context={kind:'chapter',moduleSlug:mod.module.slug,chapterSlug:'intro'}
  const start=await send('/api/targets',{actor:learner,method:'POST',body:{...context,action:'start'}});assert.equal(start.res.status,200);const instance=start.json;assert.equal(instance.state,'running');assert.equal(instance.instance.entry_address,'10.202.0.2:8080');assert.equal(instance.instance.attachment,'not_connected')
  const again=await send('/api/targets',{actor:learner,method:'POST',body:{...context,action:'start'}});assert.equal(again.json.id,instance.id);assert.equal(again.json.expiresAt,instance.expiresAt)
  const create=worker.requests.find(r=>r.type==='compose_start');assert.equal(create.payload.deployment_id,queried.json.data.deploymentId);assert.ok(!('image'in create.payload))
  if(process.env.COMPOSE_BROWSER_TEST==='1'){
   const {chromium}=await import('playwright'),browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-sandbox']})
   try{
    const adminContext=await browser.newContext({viewport:{width:1440,height:1000}});await adminContext.addCookies([{name:'attackbox_session',value:f.admin.token,url:f.base}]);const page=await adminContext.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message))
    await page.goto(f.base+'/admin/target-templates/compose/new');await page.locator('input[type=file]').setInputFiles({name:'preview.zip',mimeType:'application/zip',buffer:data});await page.getByText('1 个服务 · 2 个文件',{exact:true}).waitFor()
    await page.getByLabel('LAB_IMAGE',{exact:true}).fill('fixture:local');await page.getByLabel('PASSWORD',{exact:true}).fill('browser-private-value');await page.getByRole('button',{name:'保存模板',exact:true}).click();await page.waitForURL(/\/compose\/[a-f0-9-]{36}$/);extraTemplates.push(new URL(page.url()).pathname.split('/').at(-1));await page.getByRole('heading',{name:'Compose 靶场',exact:true}).waitFor()
    await page.getByRole('button',{name:'清除 PASSWORD 的已保存值',exact:true}).click();await page.getByRole('button',{name:'保存模板',exact:true}).click();await page.getByText('模板已保存',{exact:true}).waitFor();const cleared=await send(`/api/admin/target-templates/${extraTemplates[0]}`);assert.ok(!cleared.json.data.variableKeys.includes('PASSWORD'));assert.ok(cleared.json.data.variableKeys.includes('LAB_IMAGE'))
    await page.goto(f.base+`/admin/target-templates/compose/${template.id}`);await page.getByRole('heading',{name:'Compose 靶场',exact:true}).waitFor();await page.getByText('已准备',{exact:true}).waitFor()
    assert.equal(await page.locator('input[type=password]').count(),2);for(const input of await page.locator('input[type=password]').all())assert.equal(await input.inputValue(),'')
    const count=worker.requests.filter(r=>r.type==='compose_prepare_status').length;await page.getByRole('button',{name:'查询状态'}).click();await page.getByText('准备记录',{exact:true}).click();await page.getByText('镜像准备完成',{exact:true}).waitFor();assert.equal(worker.requests.filter(r=>r.type==='compose_prepare_status').length,count+1)
    await page.evaluate(()=>window.scrollTo(0,0));await page.screenshot({path:'/tmp/compose-template-admin.png',fullPage:true})
    const learnerContext=await browser.newContext({viewport:{width:1440,height:1000}});await learnerContext.addCookies([{name:'attackbox_session',value:learner.token,url:f.base}]);const reader=await learnerContext.newPage();reader.on('pageerror',e=>errors.push(e.message))
    await reader.goto(f.base+`/learn/modules/${mod.module.slug}/intro`);await reader.getByText('10.202.0.2:8080',{exact:true}).waitFor();await reader.getByText('服务状态',{exact:true}).click()
    await reader.getByRole('button',{name:'启动环境',exact:true}).click();await reader.locator('.target-state').filter({hasText:'已就绪'}).waitFor();await reader.getByText('10.202.0.14',{exact:true}).waitFor();assert.ok(worker.requests.some(r=>r.type==='create_box'&&r.payload.owner_id===learner.id));assert.deepEqual(errors,[])
    await reader.screenshot({path:'/tmp/compose-learning-target.png',fullPage:true})
   }finally{await browser.close()}
  }
  await send('/api/targets',{actor:other,method:'POST',body:{...context,action:'stop',id:instance.id}});assert.ok(!worker.requests.some(r=>r.type==='compose_stop'&&r.payload.target_id===instance.id))
  // A settings edit creates a new preparation revision, preserving the running instance.
  const edited=await send(`/api/admin/compose-templates/${template.id}`,{method:'PUT',body:{...body,variables:{},settings:{...body.settings,entryPort:8081},enabled:false,lockVersion:template.lockVersion}});assert.equal(edited.res.status,200);assert.equal(edited.json.data.composeRevision,2)
  const stop=await send('/api/targets',{actor:learner,method:'POST',body:{action:'stop',id:instance.id}});assert.equal(stop.res.status,200);assert.equal(stop.json.state,'stopped');assert.equal(worker.requests.find(r=>r.type==='compose_stop').payload.deployment_id,create.payload.deployment_id)
  const baseline=worker.requests.filter(r=>r.type==='compose_prepare_status').length;await send(prepPath);assert.equal(worker.requests.filter(r=>r.type==='compose_prepare_status').length,baseline,'GET must not query Worker automatically')
  const cross=await send('/api/admin/lab-bundles',{method:'POST',raw:data,headers:{'content-type':'application/octet-stream','x-lab-filename':'lab.zip',origin:'https://other.example'}});assert.equal(cross.res.status,403)
 }finally{
  const sandbox=worker?.requests.find(r=>r.type==='create_box'&&r.payload.owner_id===learner?.id)?.payload.sandbox_id
  if(sandbox)await send('/api/sandbox/stop',{actor:learner,method:'POST',body:{sandboxId:sandbox}})
  if(worker)await worker.close()
  for(const u of [learner,other])if(u)await f.sql`delete from target_instances where user_id=${u.id}`
  if(mod)await f.sql`delete from modules where id=${mod.module.id}`
  if(template)await f.sql`delete from target_templates where id=${template.id}`
  await f.sql`delete from target_templates where bundle_id in (select id from lab_bundles where actor_id=${f.admin.id})`
  for(const id of extraTemplates)await f.sql`delete from target_templates where id=${id}`
  const uploaded=await f.sql`select id from lab_bundles where actor_id=${f.admin.id}`
  for(const {id} of uploaded){await f.sql`delete from lab_bundles where id=${id}`;if(process.env.LAB_DATA_DIR)await rm(join(process.env.LAB_DATA_DIR,id),{recursive:true,force:true})}
  await f.cleanup()
 }
})
