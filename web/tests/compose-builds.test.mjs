import test from 'node:test'
import assert from 'node:assert/strict'
import {rm} from 'node:fs/promises'
import {join} from 'node:path'
import {learningFixture} from './helpers/learning-fixture.mjs'
import {sandboxWorker} from './helpers/sandbox-worker.mjs'
import {labZip} from './helpers/lab-zip.mjs'
test('Build package preview, missing Dockerfile, directory mounts and administrator workflow',{skip:!process.env.CONTENT_TEST_DATABASE_URL||!process.env.NODE_TEST_CLUSTER_SECRET,timeout:60000},async()=>{
 const f=await learningFixture();let worker,browser,template
 const yaml="services: {web: {build: {context: ./app, args: {MODE: '${MODE:-demo}'}}, volumes: ['./config:/config:ro', './data:/data']}}"
 const entries=[{name:'lab/compose.yml',data:yaml},{name:'lab/config/site.conf',data:'config'},{name:'lab/data/seed.txt',data:'seed'},{name:'lab/app/main.txt',data:'app'}]
 const upload=async entries=>{const r=await fetch(f.base+'/api/admin/lab-bundles',{method:'POST',headers:{cookie:`attackbox_session=${f.admin.token}`,'content-type':'application/octet-stream','x-lab-filename':'build-lab.zip'},body:labZip(entries)});assert.equal(r.status,200);return (await r.json()).data}
 try{
  worker=await sandboxWorker(f.base,process.env.NODE_TEST_CLUSTER_SECRET,{composeLabs:true})
  const missing=await upload(entries);assert.ok(missing.missingFiles.includes('lab/app/Dockerfile'));assert.ok(!missing.missingFiles.includes('lab/config'))
  const settings={file:'lab/compose.yml',entryService:'web',entryNetwork:'default',entryPort:8080,prefixBits:28}
  await f.request('/api/admin/compose-templates',{actor:f.admin,method:'POST',status:400,body:{name:'Invalid build',description:'',accessInstructions:'',enabled:false,bundleId:missing.id,settings,variables:{}}})
  entries.push({name:'lab/app/Dockerfile',data:'FROM scratch\nCOPY main.txt /main.txt\n'})
  const {chromium}=await import('playwright');browser=await chromium.launch({executablePath:'/usr/bin/chromium',args:['--no-sandbox']});const ctx=await browser.newContext({viewport:{width:1440,height:1000}});await ctx.addCookies([{name:'attackbox_session',value:f.admin.token,url:f.base}]);const page=await ctx.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message))
  await page.goto(f.base+'/admin/target-templates/compose/new');await page.locator('input[type=file]').setInputFiles({name:'build-lab.zip',mimeType:'application/zip',buffer:labZip(entries)});await page.locator('.build-context').filter({hasText:'本地构建 · lab/app'}).waitFor();await page.getByText('lab/app/Dockerfile',{exact:true}).waitFor();await page.getByLabel('MODE',{exact:true}).fill('browser-mode')
  await page.getByRole('button',{name:'保存模板',exact:true}).click();await page.waitForURL(/\/compose\/[a-f0-9-]{36}$/);const id=page.url().split('/').at(-1);template=(await f.request(`/api/admin/target-templates/${id}`,{actor:f.admin})).data
  const bundle=(await f.request(`/api/admin/lab-bundles/${template.bundleId}`,{actor:f.admin})).data;assert.deepEqual(bundle.issues,[]);assert.deepEqual(bundle.missingFiles,[]);assert.equal(bundle.services[0].build.context,'lab/app')
  await page.getByRole('button',{name:'准备部署',exact:true}).click();await page.getByText('准备中',{exact:true}).waitFor();await page.getByRole('button',{name:'查询状态',exact:true}).click();await page.locator('.node-row').getByText('已准备',{exact:true}).waitFor();await page.getByRole('checkbox').check();await page.getByRole('button',{name:'保存模板',exact:true}).click();await page.getByText('模板已保存',{exact:true}).waitFor()
  template=(await f.request(`/api/admin/target-templates/${id}`,{actor:f.admin})).data;assert.equal(template.enabled,true);assert.deepEqual(errors,[]);await page.evaluate(()=>window.scrollTo(0,0));await page.screenshot({path:'/tmp/attackbox-compose-build-admin.png',fullPage:true})
 }finally{
  await browser?.close();await worker?.close();await f.sql`delete from target_templates where bundle_id in (select id from lab_bundles where actor_id=${f.admin.id})`;const rows=await f.sql`select id from lab_bundles where actor_id=${f.admin.id}`;await f.sql`delete from lab_bundles where actor_id=${f.admin.id}`;for(const {id}of rows)if(process.env.LAB_DATA_DIR)await rm(join(process.env.LAB_DATA_DIR,id),{recursive:true,force:true});await f.cleanup()
 }
})
