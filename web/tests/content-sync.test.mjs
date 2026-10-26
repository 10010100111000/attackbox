import test from 'node:test'
import assert from 'node:assert/strict'
import {mkdtemp,mkdir,writeFile,rename,rm} from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import {createJiti} from 'jiti'
import {chromium} from 'playwright'
import {learningFixture} from './helpers/learning-fixture.mjs'
const enabled=!!process.env.CONTENT_TEST_DATABASE_URL&&!!process.env.CONTENT_TEST_BASE_URL

test('manual local/Git import preserves identity and progress, syncs chapter/question removals, protects images, and renders new admin pages',{skip:!enabled,timeout:120000},async()=>{
 process.env.DATABASE_URL=process.env.CONTENT_TEST_DATABASE_URL
 const f=await learningFixture(),jiti=createJiti(import.meta.url)
 const sync=await jiti.import('../server/services/content-sync.ts'),repository=await jiti.import('../server/services/content-repository.ts'),database=await jiti.import('../server/db/index.ts')
 const root=await mkdtemp(path.join(os.tmpdir(),'content-import-test-')),moduleIds=new Set();let browser,template
 const previews=[],oldFetch=globalThis.$fetch
 const makeSource=(reverse=false,first='First',prompt='Which command?',answer='whoami')=>`---\nsummary: Imported module\naccess: authenticated\nchapters:\n  ${first}:\n    access: public\n---\n\n${(reverse?[`# Second\n\nPrivate diagram: ![diagram](images/test.png)`,`# ${first}\n\nBody\n\n\`\`\`quiz\nprompt: ${prompt}\nanswer: ${answer}\n\`\`\``]:[`# ${first}\n\nBody\n\n\`\`\`quiz\nprompt: ${prompt}\nanswer: ${answer}\n\`\`\``,`# Second\n\nPrivate diagram: ![diagram](images/test.png)`]).join('\n\n')}`
 try{
  const templateResponse=await fetch(f.base+'/api/admin/content/template',{headers:{cookie:`attackbox_session=${f.admin.token}`}});assert.equal(templateResponse.status,200)
  const {parseModuleMarkdown}=await jiti.import('../server/content/parse-module.ts')
  assert.equal(parseModuleMarkdown('Downloaded.md',await templateResponse.text()).chapters.length,2)
  await mkdir(path.join(root,'images'));await writeFile(path.join(root,'images/test.png'),Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=','base64'))
  await writeFile(path.join(root,'Imported.md'),makeSource())
  const create=async()=>{const p=await sync.createLocalContentPreview(root,f.admin.id);previews.push(p.id);return p}
  let preview=await create();assert.equal(preview.conflicts.length,0);assert.ok(!JSON.stringify(preview).includes('answer: whoami'))
  const learner=await f.account(),otherAdmin=await f.account('admin')
  await f.request(`/api/admin/content/previews/${preview.id}`,{actor:otherAdmin,status:404})
  await f.request('/api/admin/content/repository',{actor:learner,status:403})
  const result=await sync.applyContentPreview(preview.id,f.admin.id);result.moduleIds.forEach(id=>moduleIds.add(id))
  assert.deepEqual(await sync.applyContentPreview(preview.id,f.admin.id),result,'same confirmation is idempotent')
  const id=result.moduleIds[0]
  let detail=(await f.request(`/api/admin/modules/${id}`,{actor:f.admin})).data
  assert.equal(detail.module.status,'unpublished');assert.equal(detail.chapters.length,2);assert.equal(detail.chapters[0].questions[0].answer,null,'Git answers are not returned by the admin details API')
  const chapterId=detail.chapters[0].chapterId,chapterSlug=detail.chapters[0].slug,questionId=detail.chapters[0].questions[0].id,moduleSlug=detail.module.slug
  await f.request(`/api/admin/modules/${id}/runtime`,{actor:f.admin,method:'PUT',body:{lockVersion:detail.module.lockVersion,descriptionMarkdown:'cannot change'},status:400})
  await f.request(`/api/admin/modules/${id}/runtime`,{actor:f.admin,method:'PUT',body:{lockVersion:detail.module.lockVersion,chapters:[null]},status:400})
  const retiredPut=await fetch(`${f.base}/api/admin/modules/${id}`,{method:'PUT',headers:{cookie:`attackbox_session=${f.admin.token}`,'content-type':'application/json'},body:JSON.stringify({title:'Retired',lockVersion:detail.module.lockVersion})})
  assert.ok([404,405].includes(retiredPut.status),'legacy full-module PUT endpoint is retired')
  await f.request(`/api/admin/modules/${id}/publish`,{actor:f.admin,method:'POST',body:{lockVersion:detail.module.lockVersion}})
  const attempt=await fetch(`${f.base}/api/modules/${moduleSlug}/chapters/${chapterSlug}/attempt`,{method:'POST',headers:{cookie:`attackbox_session=${learner.token}`,'content-type':'application/json'},body:JSON.stringify({questionId,answer:'whoami'})});assert.equal(attempt.status,200)
  preview=await create();assert.equal(preview.modules[0].changed,false)
  await sync.applyContentPreview(preview.id,f.admin.id)
  const privateChapter=(await f.request(`/api/admin/modules/${id}`,{actor:f.admin})).data.chapters[1]
  const privateData=(await f.request(`/api/modules/${moduleSlug}/chapters/${privateChapter.slug}`,{actor:learner})).data
  const imageUrl=Object.values(privateData.imageUrls)[0];assert.ok(imageUrl)
  assert.equal((await fetch(f.base+imageUrl)).status,401)
  assert.equal((await fetch(f.base+imageUrl,{headers:{cookie:`attackbox_session=${learner.token}`}})).status,200)
  template=await f.seedTarget({name:'Import fixture target',description:'',image:'nginx:1.27',sourceUrl:'',ports:[],accessInstructions:'',enabled:true})
  detail=(await f.request(`/api/admin/modules/${id}`,{actor:f.admin})).data
  const contentBefore=detail.chapters.map(c=>({id:c.chapterId,slug:c.slug,position:c.position,title:c.title,body:c.bodyMarkdown,questions:c.questions.map(q=>q.id)}))
  const runtimePath=`/api/admin/modules/${id}/runtime`,runtimeBody={lockVersion:detail.module.lockVersion,defaultTargetTemplateId:template.id,chapters:[{chapterId,targetMode:'none',targetTemplateId:null}]}
  await f.request(runtimePath,{actor:f.admin,method:'PUT',body:runtimeBody})
  const afterRuntime=(await f.request(`/api/admin/modules/${id}`,{actor:f.admin})).data
  assert.deepEqual(afterRuntime.chapters.map(c=>({id:c.chapterId,slug:c.slug,position:c.position,title:c.title,body:c.bodyMarkdown,questions:c.questions.map(q=>q.id)})),contentBefore,'runtime save never rewrites tutorial content, order or question identity')
  assert.equal(afterRuntime.module.lockVersion,detail.module.lockVersion+1);assert.equal(afterRuntime.module.defaultTargetTemplateId,template.id);assert.equal(afterRuntime.chapters.find(c=>c.chapterId===chapterId).targetMode,'none')
  await f.request(runtimePath,{actor:f.admin,method:'PUT',body:runtimeBody,status:409})
  const current=afterRuntime.module.lockVersion
  await f.request(runtimePath,{actor:f.admin,method:'PUT',body:{lockVersion:current,chapters:[{chapterId,targetMode:'bogus'}]},status:400})
  await f.request(runtimePath,{actor:f.admin,method:'PUT',body:{lockVersion:current,chapters:[{chapterId:'00000000-0000-4000-8000-000000000000',targetMode:'none'}]},status:409})
  await f.request(runtimePath,{actor:f.admin,method:'PUT',body:{lockVersion:'1'},status:400})
  await f.request(runtimePath,{actor:f.admin,method:'PUT',body:{lockVersion:current,prerequisiteModuleIds:[id]},status:400})
  assert.equal((await f.request(`/api/admin/modules/${id}`,{actor:f.admin})).data.module.lockVersion,current,'rejected runtime saves roll back completely')
  detail=afterRuntime
  await writeFile(path.join(root,'Imported.md'),makeSource(true,'First','Which command?','changed-answer'))
  preview=await create();await sync.applyContentPreview(preview.id,f.admin.id)
  detail=(await f.request(`/api/admin/modules/${id}`,{actor:f.admin})).data
  assert.equal(detail.module.status,'published');assert.equal(detail.module.defaultTargetTemplateId,template.id)
  assert.equal(detail.chapters[1].chapterId,chapterId);assert.equal(detail.chapters[1].targetMode,'none');assert.equal(detail.chapters[1].questions[0].id,questionId)
  const [progress]=await f.sql`select completed_at from chapter_progress where chapter_id=${chapterId} and user_id=${learner.id}`;assert.ok(progress.completed_at)
  assert.equal((await f.sql`select * from question_completions where question_id=${questionId} and user_id=${learner.id}`).length,1)
  await rename(path.join(root,'Imported.md'),path.join(root,'Renamed.md'))
  await writeFile(path.join(root,'Renamed.md'),makeSource(true,'Renamed chapter','Renamed prompt','changed-answer'))
  preview=await create()
  preview=await sync.updateContentBindings(preview.id,f.admin.id,{modules:{'Renamed.md':id},chapters:{[sync.bindingKey('Renamed.md','Renamed chapter')]:chapterId},questions:{[sync.bindingKey('Renamed.md','Renamed chapter','Renamed prompt')]:questionId}})
  assert.equal(preview.conflicts.length,0);assert.equal(preview.removals.length,0)
  await sync.applyContentPreview(preview.id,f.admin.id)
  detail=(await f.request(`/api/admin/modules/${id}`,{actor:f.admin})).data
  assert.equal(detail.module.slug,moduleSlug);assert.equal(detail.module.title,'Renamed');assert.equal(detail.chapters[1].slug,chapterSlug)
  // Unbound removed chapters/questions report in removals without blocking apply.
  await writeFile(path.join(root,'Renamed.md'),'---\n---\n# Second\n\nBody')
  preview=await create();assert.equal(preview.conflicts.length,0);assert.ok(preview.removals.length>0)
  await f.request(`/api/admin/modules/${id}/unpublish`,{actor:f.admin,method:'POST',body:{lockVersion:detail.module.lockVersion}})
  await assert.rejects(sync.applyContentPreview(preview.id,f.admin.id),/已变化/)
  preview=await create();assert.equal(preview.conflicts.length,0);assert.ok(preview.removals.length>0)
  await sync.applyContentPreview(preview.id,f.admin.id)
  detail=(await f.request(`/api/admin/modules/${id}`,{actor:f.admin})).data
  assert.equal(detail.chapters.length,1);assert.equal(detail.chapters[0].title,'Second')
  assert.equal((await f.sql`select id from chapters where id=${chapterId}`).length,0,'removed chapter is deleted on sync confirmation')
  const settings=(await f.request('/api/admin/content/repository',{actor:f.admin})).data
  const rawToken='PRIVATE_IMPORT_TEST_TOKEN'
  await f.request('/api/admin/content/repository',{actor:f.admin,method:'PUT',body:{repository:'example/tutorials',branch:'main',directory:'courses',token:rawToken,version:settings.version}})
  const publicSettings=(await f.request('/api/admin/content/repository',{actor:f.admin})).data
  assert.equal(publicSettings.hasToken,true);assert.ok(!JSON.stringify(publicSettings).includes(rawToken))
  const [stored]=await f.sql`select token_cipher from content_repository where id=1`;assert.ok(stored.token_cipher&&!stored.token_cipher.includes(rawToken))
  const remote='---\n---\n# Remote chapter\n\nRemote body'
  let calls=0
  globalThis.$fetch=async(url,options)=>{
   calls++;assert.equal(options.headers.Authorization,`Bearer ${rawToken}`)
   assert.ok(url.startsWith('https://api.github.com/repos/example/tutorials/'))
   if(url.endsWith('/commits/main'))return {sha:'a'.repeat(40)}
   if(url.includes('/git/commits/'))return {tree:{sha:'root'}}
   if(url.endsWith('/git/trees/root'))return {tree:[{path:'outside.md',type:'blob',sha:'not-read'},{path:'courses',type:'tree',sha:'courses'}]}
   if(url.endsWith('/git/trees/courses?recursive=1'))return {tree:[{type:'blob',mode:'100644',path:'Remote.md',size:Buffer.byteLength(remote),sha:'blob'}]}
   if(url.endsWith('/git/blobs/blob'))return {encoding:'base64',content:Buffer.from(remote).toString('base64')}
   throw Error('Unexpected endpoint')
  }
  let remotePreview=await repository.pullContentRepository(f.admin.id);previews.push(remotePreview.id)
  assert.equal(remotePreview.commit,'a'.repeat(40));assert.equal(remotePreview.modules.length,1);assert.equal(calls,5)
  await f.request('/api/admin/content/repository',{actor:f.admin,method:'PUT',body:{repository:'example/tutorials',branch:'main',directory:'courses',token:'',version:publicSettings.version}})
  await f.request(`/api/admin/content/previews/${remotePreview.id}/apply`,{actor:f.admin,method:'POST',status:409})
  assert.equal((await f.request(`/api/admin/content/previews/${remotePreview.id}`,{actor:f.admin})).data.stale,true)
  remotePreview=await repository.pullContentRepository(f.admin.id);previews.push(remotePreview.id);assert.equal(calls,10)
  const appliedRemote=await f.request(`/api/admin/content/previews/${remotePreview.id}/apply`,{actor:f.admin,method:'POST'});appliedRemote.data.moduleIds.forEach(id=>moduleIds.add(id));const remoteDetail=(await f.request(`/api/admin/modules/${appliedRemote.data.moduleIds[0]}`,{actor:f.admin})).data;assert.equal(remoteDetail.source.url,`https://github.com/example/tutorials/blob/${'a'.repeat(40)}/courses/Remote.md`);assert.equal(calls,10,'confirmation does not read a new GitHub HEAD')
  await f.request(`/api/admin/content/previews/${preview.id}`,{actor:f.admin,method:'DELETE'})
  await f.request(`/api/admin/content/previews/${preview.id}`,{actor:f.admin,status:404})
  browser=await chromium.launch({executablePath:'/usr/bin/chromium',args:['--no-sandbox']})
  const context=await browser.newContext({viewport:{width:1600,height:1000}});await context.addCookies([{name:'attackbox_session',value:f.admin.token,url:f.base}])
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message))
  await page.goto(`${f.base}/admin/modules/${id}`);await page.locator('.git-module-detail').waitFor();assert.equal(await page.locator('.editor-double-layout').count(),0)
  await page.screenshot({path:'/tmp/git-module-detail.png'})
  await page.getByRole('link',{name:'Git 同步',exact:true}).click();await page.getByRole('heading',{name:'Git 教程同步',exact:true}).waitFor()
  await page.locator('input[type=password]').waitFor();assert.equal(await page.locator('input[type=password]').inputValue(),'')
  await page.screenshot({path:'/tmp/git-content-admin.png'})
  assert.deepEqual(errors,[])
 }finally{
  globalThis.$fetch=oldFetch
  if(browser)await browser.close()
  await f.sql`delete from content_previews where actor_id=${f.admin.id}`
  for(const id of moduleIds){await f.sql`delete from audit_logs where target_id=${id}`;await f.sql`delete from modules where id=${id}`}
  await f.sql`delete from audit_logs where actor_user_id=${f.admin.id}`
  await f.sql`delete from assets where uploaded_by=${f.admin.id}`
  if(template)await f.sql`delete from target_templates where id=${template.id}`
  await f.sql`delete from content_repository where id=1`
  await f.cleanup();await database.client.end();await rm(root,{recursive:true,force:true})
 }
})
