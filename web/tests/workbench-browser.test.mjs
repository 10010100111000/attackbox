import test from 'node:test'
import assert from 'node:assert/strict'
import {chromium} from 'playwright'
import {learningFixture} from './helpers/learning-fixture.mjs'
import {sandboxWorker} from './helpers/sandbox-worker.mjs'
const wait=async fn=>{for(let i=0;i<100;i++){if(await fn())return;await new Promise(r=>setTimeout(r,100))}throw Error('Timed out')}
test('workbenches reuse environment, preserve CLI windows, transfer GUI and expose release outside dock',{skip:!process.env.CONTENT_TEST_DATABASE_URL||!process.env.NODE_TEST_CLUSTER_SECRET,timeout:90000},async()=>{
 const f=await learningFixture();let worker,browser
 try{
  worker=await sandboxWorker(f.base,process.env.NODE_TEST_CLUSTER_SECRET)
  const alice=await f.account(),mod=await f.createModule()
  const instance=(await f.request('/api/sandbox/start',{actor:alice,method:'POST'})).data
  browser=await chromium.launch({executablePath:'/usr/bin/chromium',args:['--no-sandbox']});const context=await browser.newContext({viewport:{width:1600,height:1000}})
  await context.addCookies([{name:'attackbox_session',value:alice.token,url:f.base}])
  const errors=[];context.on('page',p=>p.on('pageerror',e=>errors.push(e.message)))
  const page=await context.newPage();await page.goto(`${f.base}/learn/modules/${mod.module.slug}/intro`)
  await page.locator('.environment-status time').waitFor()
  await page.getByRole('button',{name:'CLI 终端',exact:true}).click();await page.locator('.terminal-status').filter({hasText:'已连接'}).waitFor()
  const popup=context.waitForEvent('page');await page.getByRole('button',{name:'独立打开',exact:true}).click();const cli=await popup
  await cli.waitForURL('**/term');await cli.locator('.terminal-status').filter({hasText:'已连接'}).waitFor()
  await cli.getByRole('button',{name:'新建终端',exact:true}).click();await wait(()=>cli.locator('.terminal-window').count().then(n=>n===2))
  await cli.getByRole('button',{name:'平铺',exact:true}).click();const first=await cli.locator('.terminal-window').first().boundingBox();assert.ok(first.width>700)
  const windows=await cli.locator('.terminal-window').count(),terminals=worker.terminals.length
  await page.bringToFront();await page.getByRole('button',{name:'独立打开',exact:true}).click();await cli.bringToFront();assert.equal(await cli.locator('.terminal-window').count(),windows);assert.equal(worker.terminals.length,terminals)
  await page.bringToFront();await page.getByRole('button',{name:'GUI 桌面',exact:true}).click();await page.getByText('桌面已连接',{exact:true}).waitFor()
  const guiPopup=context.waitForEvent('page');await page.getByRole('button',{name:'独立打开',exact:true}).click();const gui=await guiPopup
  await gui.waitForURL('**/gui');await gui.getByText('桌面已连接',{exact:true}).waitFor();await page.getByText('桌面已在其他页面打开',{exact:true}).waitFor()
  assert.equal(await gui.locator('.environment-workbench').evaluate(e=>e.clientHeight),1000)
  await page.bringToFront();await page.getByRole('button',{name:'在此继续操作',exact:true}).click();await page.getByText('桌面已连接',{exact:true}).waitFor();await gui.getByText('桌面已在其他页面操作',{exact:true}).waitFor()
  await page.getByRole('button',{name:'收起练习面板',exact:true}).click()
  await page.getByRole('button',{name:'环境操作',exact:true}).click();page.once('dialog',d=>d.accept());await page.getByRole('menuitem',{name:'释放环境',exact:true}).click()
  await page.getByRole('button',{name:'启动环境',exact:true}).waitFor();await wait(()=>!worker.boxes.has(instance.id));assert.equal(worker.requests.filter(r=>r.type==='create_box').length,1)
  assert.deepEqual(errors,[])
 }finally{if(browser)await browser.close();if(worker)await worker.close();await f.cleanup()}
})
