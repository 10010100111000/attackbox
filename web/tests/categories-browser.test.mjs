import assert from 'node:assert/strict'
import {chromium} from 'playwright'
import test from 'node:test'
import {learningFixture} from './helpers/learning-fixture.mjs'
test('category drawer preserves editor geometry and supports management', {skip:!process.env.CONTENT_TEST_DATABASE_URL}, async()=>{
const f=await learningFixture();let browser,category
try{
 const mod=await f.createModule();browser=await chromium.launch({executablePath:'/usr/bin/chromium',args:['--no-sandbox']})
 const context=await browser.newContext({viewport:{width:1600,height:1000}});await context.addCookies([{name:'attackbox_session',value:f.admin.token,url:f.base}])
 const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message))
 await page.goto(`${f.base}/admin/modules/${mod.module.id}?tab=basic`)
 const trigger=page.locator('#module-category');await trigger.waitFor();await page.waitForTimeout(200)
 const before=await page.locator('#module-summary').boundingBox();await trigger.click();await page.locator('.category-drawer').waitFor()
 const after=await page.locator('#module-summary').boundingBox();assert.deepEqual(after,before)
 await page.screenshot({path:'/tmp/attackbox-category-drawer.png'})
 await page.getByRole('button',{name:'新建',exact:true}).click();const name='Browser '+crypto.randomUUID().slice(0,8)
 await page.locator('#category-name').fill(name);await page.getByRole('button',{name:'保存',exact:true}).click()
 const created=(await f.request('/api/admin/categories',{actor:f.admin})).data.find(c=>c.name===name);assert.ok(created);category=created
 await page.getByRole('button',{name:`重命名 ${name}`,exact:true}).click();await page.locator('#category-name').fill(name+' renamed');await page.getByRole('button',{name:'保存',exact:true}).click()
 await page.getByRole('button',{name:`删除 ${name} renamed`,exact:true}).click();await page.getByRole('button',{name:'确认删除',exact:true}).click();await page.getByRole('button',{name:'关闭分类管理'}).click()
 assert.match(await trigger.innerText(),/未分类/);assert.deepEqual(errors,[])
 // A saved category removal must not discard unrelated in-progress edits.
 const linked=(await f.request('/api/admin/categories',{actor:f.admin,method:'POST',body:{name:name+' linked'}})).data
 category=linked
 await f.sql`update modules set category_id=${linked.id},lock_version=lock_version+1 where id=${mod.module.id}`
 await page.reload();await page.locator('#module-title').waitFor()
 const draftTitle='Preserved '+name
 await page.locator('#module-title').fill(draftTitle)
 await trigger.click();await page.getByRole('button',{name:`删除 ${linked.name}`,exact:true}).click()
 await page.getByRole('button',{name:'确认删除',exact:true}).click();await page.getByRole('button',{name:'关闭分类管理'}).click()
 assert.equal(await page.locator('#module-title').inputValue(),draftTitle)
 const saved=page.waitForResponse(r=>r.url().endsWith(`/api/admin/modules/${mod.module.id}`)&&r.request().method()==='PUT')
 await page.getByRole('button',{name:'保存并发布',exact:true}).click();assert.equal((await saved).status(),200)
 const [stored]=await f.sql`select category_id,title,status from modules where id=${mod.module.id}`
 assert.equal(stored.category_id,null);assert.equal(stored.title,draftTitle);assert.equal(stored.status,'published')
 assert.deepEqual(errors,[])
 console.log('Drawer geometry, CRUD, atomic detach and preservation of unsaved edits passed')
}finally{if(browser)await browser.close();if(category){await f.sql`update modules set category_id=null where category_id=${category.id}`;await f.sql`delete from content_categories where id=${category.id}`};await f.cleanup()}

})
