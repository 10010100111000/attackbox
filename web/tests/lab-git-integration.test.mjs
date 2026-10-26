import test from 'node:test'
import assert from 'node:assert/strict'
import {rm} from 'node:fs/promises'
import {join} from 'node:path'
import {learningFixture} from './helpers/learning-fixture.mjs'
import {sandboxWorker} from './helpers/sandbox-worker.mjs'
test('GitHub import, explicit update confirmation and same-content revision reuse in browser',{skip:!process.env.LAB_GIT_LIVE_TEST||!process.env.CONTENT_TEST_DATABASE_URL,timeout:180000},async()=>{
 const f=await learningFixture();let worker,browser,template
 try{
  const {chromium}=await import('playwright');worker=await sandboxWorker(f.base,process.env.NODE_TEST_CLUSTER_SECRET,{composeLabs:true})
  await f.request('/api/admin/lab-bundles/git',{method:'POST',body:{repository:'a/b'},status:401})
  const learner=await f.account();await f.request('/api/admin/lab-bundles/git',{actor:learner,method:'POST',body:{repository:'a/b'},status:403})
  await f.request('/api/admin/lab-bundles/git',{actor:f.admin,method:'POST',body:{repository:'https://evil.example/repo'},status:400})
  browser=await chromium.launch({executablePath:'/usr/bin/chromium',args:['--no-sandbox']});const ctx=await browser.newContext({viewport:{width:1440,height:1000}});await ctx.addCookies([{name:'attackbox_session',value:f.admin.token,url:f.base}]);const p=await ctx.newPage(),errors=[];p.on('pageerror',e=>errors.push(e.message))
  await p.goto(f.base+'/admin/target-templates/new');await p.getByRole('button',{name:'从 GitHub 导入',exact:true}).click();await p.getByLabel('GitHub 仓库',{exact:true}).fill('10010100111000/attackbox');await p.getByLabel('仓库内目录',{exact:true}).fill('node/testdata/compose-lab')
  await p.getByRole('button',{name:'导入并预览',exact:true}).click();await p.getByText('已导入，请确认部署入口并保存模板。',{exact:true}).waitFor({timeout:120000});await p.getByLabel('LAB_IMAGE',{exact:true}).fill('fixture:local');await p.getByRole('button',{name:'保存模板',exact:true}).click();await p.waitForURL(/\/target-templates\/[a-f0-9-]{36}$/)
  const id=p.url().split('/').at(-1);template=(await f.request(`/api/admin/target-templates/${id}`,{actor:f.admin})).data
  const originalBundle=template.bundleId
  const source=(await f.request(`/api/admin/lab-bundles/${originalBundle}`,{actor:f.admin})).data.source
  assert.equal(source.repository,'10010100111000/attackbox');assert.match(source.commit,/^[a-f0-9]{40}$/);assert.equal(source.hasToken,false)
  await f.request(`/api/admin/target-templates/${id}/preparations`,{actor:f.admin,method:'POST',body:{nodeId:'test-node',action:'prepare'}})
  await f.request(`/api/admin/target-templates/${id}/preparations`,{actor:f.admin,method:'POST',body:{nodeId:'test-node',action:'status'}})
  const body={...template,variables:{},bundleId:originalBundle,settings:template.settings,enabled:true}
  template=(await f.request(`/api/admin/target-templates/${id}`,{actor:f.admin,method:'PUT',body})).data
  // Model a previous commit with identical files: metadata updates must not invalidate preparation.
  await f.sql`update lab_bundles set source=jsonb_set(source,'{commit}',${JSON.stringify('a'.repeat(40))}::jsonb) where id=${originalBundle}`
  await p.reload();await p.getByRole('heading',{name:'Compose 靶场',exact:true}).waitFor()
  const checks=[];p.on('request',r=>{if(r.url().endsWith('/git'))checks.push(r)})
  await p.getByRole('button',{name:'检查更新',exact:true}).click();await p.getByText('发现新提交，确认后可导入更新。',{exact:true}).waitFor({timeout:30000})
  await p.getByRole('button',{name:'确认导入更新',exact:true}).click();await p.getByText('提交已更新，文件内容相同，无需重新准备；保存后记录新来源。',{exact:true}).waitFor({timeout:120000})
  await p.getByRole('button',{name:'保存模板',exact:true}).click();await p.getByText('模板已保存',{exact:true}).waitFor()
  template=(await f.request(`/api/admin/target-templates/${id}`,{actor:f.admin})).data
  assert.notEqual(template.bundleId,originalBundle);assert.equal(template.revision,1);assert.equal(template.enabled,true);assert.equal(checks.length,2)
  const current=(await f.request(`/api/admin/lab-bundles/${template.bundleId}`,{actor:f.admin})).data
  assert.equal(current.source.commit,source.commit)
  await f.request(`/api/admin/lab-bundles/${template.bundleId}/git`,{actor:f.admin,method:'POST',body:{action:'import',expectedCommit:'0'.repeat(40)},status:409})
  assert.deepEqual(errors,[]);await p.evaluate(()=>window.scrollTo(0,0));await p.screenshot({path:'/tmp/attackbox-lab-git-admin.png',fullPage:true})
 }finally{
  await browser?.close();await worker?.close();if(template)await f.sql`delete from target_templates where id=${template.id}`
  const rows=await f.sql`select id from lab_bundles where actor_id=${f.admin.id}`;await f.sql`delete from target_templates where bundle_id in (select id from lab_bundles where actor_id=${f.admin.id})`;await f.sql`delete from lab_bundles where actor_id=${f.admin.id}`
  for(const {id}of rows)if(process.env.LAB_DATA_DIR)await rm(join(process.env.LAB_DATA_DIR,id),{recursive:true,force:true})
  await f.cleanup()
 }
})
