import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import {chromium} from 'playwright'
import {learningFixture} from './helpers/learning-fixture.mjs'
test('learning target bindings: inheritance, exceptions, disabled retention, access control and browser',{skip:!process.env.CONTENT_TEST_DATABASE_URL,timeout:120000},async()=>{
 const f=await learningFixture(),templates=[],modules=[];let browser
 const req=async(p,o)=>(await f.request(p,o)).data
 try{
  const user=await f.account()
  for(const name of ['模块教学模板','章节教学模板']){const t=await req('/api/admin/target-templates',{actor:f.admin,method:'POST',body:{name,description:'预置教学文件',image:'private.example/lab:1',sourceUrl:'https://private.example/project',enabled:true,ports:[{port:80,protocol:'tcp'}],accessInstructions:'查看 /home/box/materials'}});templates.push(t)}
  const [a,b]=templates
  await f.request('/api/admin/target-templates/options',{status:401});await f.request('/api/admin/target-templates/options',{actor:user,status:403})
  let d=await f.createModule();modules.push(d.module.id)
  const adminPath=`/api/admin/modules/${d.module.id}`,chapterPath=`/api/modules/${d.module.slug}/chapters/`
  const save=async body=>{const current=await req(adminPath,{actor:f.admin});return req(adminPath,{actor:f.admin,method:'PUT',body:{lockVersion:current.module.lockVersion,...body}})}
  assert.equal((await req(chapterPath+'intro')).target,null)
  await save({defaultTargetTemplateId:a.id,chapters:d.chapters.map((c,i)=>({...c,targetMode:i===2?'none':i===3?'custom':'inherit',targetTemplateId:i===3?b.id:null}))})
  let target=(await req(chapterPath+'intro')).target;assert.equal(target.name,a.name);assert.equal(target.scope,'module')
  for(const field of ['image','sourceUrl','lockVersion','enabled'])assert.ok(!(field in target))
  assert.equal((await req(chapterPath+'preview')).target,null)
  await f.request(chapterPath+'navigation',{status:401});await f.request(chapterPath+'permissions',{actor:user,status:403})
  d=await req(adminPath,{actor:f.admin});await save({chapters:d.chapters.map(c=>({...c,access:'public'}))})
  target=(await req(chapterPath+'permissions')).target;assert.equal(target.name,b.name);assert.equal(target.scope,'chapter')
  await save({summary:'普通编辑保留关联'})
  assert.equal((await req(chapterPath+'intro')).target.name,a.name)
  await f.request(adminPath,{actor:f.admin,method:'PUT',body:{lockVersion:(await req(adminPath,{actor:f.admin})).module.lockVersion,defaultTargetTemplateId:'bad'},status:400})
  await f.request(adminPath,{actor:f.admin,method:'PUT',body:{lockVersion:(await req(adminPath,{actor:f.admin})).module.lockVersion,defaultTargetTemplateId:crypto.randomUUID()},status:400})
  await req(`/api/admin/target-templates/${a.id}`,{actor:f.admin,method:'PUT',body:{...a,enabled:false}})
  assert.deepEqual((await req(chapterPath+'intro')).target,{status:'unavailable'})
  d=await req(adminPath,{actor:f.admin});await save({defaultTargetTemplateId:a.id,chapters:d.chapters})
  const other=await f.createModule();modules.push(other.module.id)
  await f.request(`/api/admin/modules/${other.module.id}`,{actor:f.admin,method:'PUT',body:{lockVersion:other.module.lockVersion,defaultTargetTemplateId:a.id},status:400})
  d=await req(adminPath,{actor:f.admin})
  await f.request(adminPath,{actor:f.admin,method:'PUT',body:{lockVersion:d.module.lockVersion,chapters:d.chapters.map((c,i)=>({...c,targetMode:i===2?'custom':c.targetMode,targetTemplateId:i===2?a.id:c.targetTemplateId}))},status:400})
  for(const patch of [{targetMode:'invalid',targetTemplateId:null},{targetMode:'custom',targetTemplateId:null},{targetMode:'inherit',targetTemplateId:b.id}]){
   const current=await req(adminPath,{actor:f.admin})
   await f.request(adminPath,{actor:f.admin,method:'PUT',body:{lockVersion:current.module.lockVersion,chapters:current.chapters.map((c,i)=>i?c:{...c,...patch})},status:400})
   assert.equal((await req(adminPath,{actor:f.admin})).module.lockVersion,current.module.lockVersion)
  }
  await req(`/api/admin/target-templates/${b.id}`,{actor:f.admin,method:'PUT',body:{...b,enabled:false}})
  d=await req(adminPath,{actor:f.admin});await save({chapters:d.chapters})
  assert.deepEqual((await req(chapterPath+'permissions')).target,{status:'unavailable'})
  await req(`/api/admin/target-templates/${b.id}`,{actor:f.admin,method:'PUT',body:{...b,lockVersion:b.lockVersion+1,enabled:true}})
  await req(`/api/admin/target-templates/${a.id}`,{actor:f.admin,method:'PUT',body:{...a,lockVersion:a.lockVersion+1,enabled:true}})
  browser=await chromium.launch({executablePath:'/usr/bin/chromium',args:['--no-sandbox']})
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});await ctx.addCookies([{name:'attackbox_session',value:f.admin.token,url:f.base}])
  const p=await ctx.newPage();await p.goto(f.base+`/admin/modules/${d.module.id}`)
  await p.getByLabel('默认靶机模板',{exact:true}).selectOption(b.id)
  await p.getByRole('button',{name:'保存并发布',exact:true}).click();await p.getByText('已保存并发布',{exact:true}).waitFor()
  assert.equal((await req(chapterPath+'intro')).target.name,b.name)
  await p.goto(f.base+`/admin/modules/${d.module.id}?tab=chapters`)
  await p.getByLabel('实验环境',{exact:true}).selectOption('custom')
  await p.getByLabel('章节靶机模板',{exact:true}).selectOption(a.id)
  await p.getByRole('button',{name:'保存并发布',exact:true}).click();await p.getByText('已保存并发布',{exact:true}).waitFor()
  assert.equal((await req(chapterPath+'intro')).target.name,a.name)
  await p.goto(f.base+`/learn/modules/${d.module.slug}/intro`)
  const box=p.locator('.target-runtime');await box.waitFor();await box.locator('summary').click()
  await p.getByText('查看 /home/box/materials',{exact:true}).waitFor()
  assert.equal(await box.locator('.target-details').getByRole('button').count(),0)
  const rect=await box.boundingBox(),body=await p.locator('.article-body > .markdown-body').boundingBox(),quiz=await p.locator('.quiz-card').boundingBox()
  assert.ok(rect.y>=body.y+body.height,'target follows chapter prose')
  if(quiz)assert.ok(rect.y+rect.height<=quiz.y,'target precedes chapter questions')
  assert.ok(!(await box.innerText()).includes('private.example'))
  await p.screenshot({path:'/tmp/attackbox-learning-target.png',fullPage:true})
  await p.goto(f.base+`/learn/modules/${d.module.slug}/preview`);await p.getByRole('heading',{name:'公开预览',exact:true}).waitFor();assert.equal(await p.locator('.target-runtime').count(),0)
  await req(`/api/admin/modules/${d.module.id}/unpublish`,{actor:f.admin,method:'POST',body:{lockVersion:(await req(adminPath,{actor:f.admin})).module.lockVersion}})
  await f.request(chapterPath+'intro',{status:404})
 }finally{await browser?.close();for(const id of modules)await f.sql`delete from modules where id=${id}`;for(const t of templates)await f.sql`delete from target_templates where id=${t.id}`;await f.cleanup()}
})
