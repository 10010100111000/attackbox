import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import {chromium} from 'playwright'
import {learningFixture} from './helpers/learning-fixture.mjs'
test('target templates: admin permissions, validation, pagination, CAS and real browser workflow',{skip:!process.env.CONTENT_TEST_DATABASE_URL,timeout:120000},async()=>{
 const f=await learningFixture(),ids=[];let browser
 const prefix='Template-'+crypto.randomUUID(),payload={name:prefix,description:'教学环境',image:'ghcr.io/example/lab:1.0',sourceUrl:'https://github.com/example/lab',ports:[{port:80,protocol:'tcp'}],accessInstructions:'访问 /training',enabled:true}
 const list='/api/admin/target-templates',req=async(p,o)=>(await f.request(p,o)).data
 const save=(id,body,status=200)=>req(id?`${list}/${id}`:list,{actor:f.admin,method:id?'PUT':'POST',body,status})
 try{
  const user=await f.account()
  await f.request(list,{status:401});await f.request(list,{actor:user,status:403})
  await f.request(list+'/invalid',{actor:f.admin,status:400})
  await f.request(`${list}/${crypto.randomUUID()}`,{actor:f.admin,status:404})
  await f.request(list+'?page=0',{actor:f.admin,status:400})
  for(const body of [{...payload,name:''},{...payload,image:'nginx'},{...payload,sourceUrl:'javascript:alert(1)'},{...payload,ports:[{port:80,protocol:'tcp'},{port:80,protocol:'tcp'}]}])await save(null,body,400)
  const created=await save(null,payload);ids.push(created.id)
  await f.request(`${list}/${created.id}`,{actor:user,status:403})
  const csrf=await fetch(f.base+`${list}/${created.id}`,{method:'PUT',headers:{cookie:`attackbox_session=${f.admin.token}`,'content-type':'application/json',origin:'https://other.invalid'},body:JSON.stringify({...payload,lockVersion:1})});assert.equal(csrf.status,403)
  const updates=await Promise.all([false,true].map(enabled=>fetch(f.base+`${list}/${created.id}`,{method:'PUT',headers:{cookie:`attackbox_session=${f.admin.token}`,'content-type':'application/json'},body:JSON.stringify({...payload,enabled,lockVersion:1})})))
  assert.deepEqual(updates.map(r=>r.status).sort(),[200,409])
  assert.equal((await req(`${list}/${created.id}`,{actor:f.admin})).lockVersion,2)
  for(let i=0;i<20;i++){const [row]=await f.sql`insert into target_templates(name,image) values(${prefix+'-'+i},'nginx:1.27') returning id`;ids.push(row.id)}
  const first=await req(list+'?search='+prefix,{actor:f.admin}),second=await req(list+'?search='+prefix+'&page=2',{actor:f.admin})
  assert.equal(first.total,21);assert.equal(first.items.length,20);assert.equal(second.items.length,1);assert.ok(!first.items.some(x=>x.id===second.items[0].id))
  assert.equal((await req(list+'?search=%25',{actor:f.admin})).total,0)
  browser=await chromium.launch({executablePath:'/usr/bin/chromium',args:['--no-sandbox']})
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});await ctx.addCookies([{name:'attackbox_session',value:f.admin.token,url:f.base}])
  const p=await ctx.newPage(),pageErrors=[];p.on('pageerror',e=>pageErrors.push(e.message))
  await p.goto(f.base+'/admin/target-templates/new');await p.getByRole('heading',{name:'新建模板',exact:true}).waitFor()
  assert.equal((await f.sql`select count(*)::int as count from target_templates`)[0].count,21)
  await p.getByLabel('名称',{exact:false}).fill('Browser '+prefix)
  await p.getByLabel('镜像引用',{exact:false}).fill('nginx')
  await p.getByRole('button',{name:'保存模板',exact:true}).click();await p.getByText('请检查标记的配置项',{exact:true}).waitFor()
  await p.getByLabel('镜像引用',{exact:false}).fill('nginx:1.27')
  await p.getByRole('button',{name:'添加端口',exact:true}).click()
  const response=p.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname===list)
  await p.getByRole('button',{name:'保存模板',exact:true}).click()
  const row=(await(await response).json()).data;ids.push(row.id)
  await p.waitForURL(`**/admin/target-templates/${row.id}?created=1`)
  await p.getByRole('checkbox').uncheck();await p.getByRole('button',{name:'保存模板',exact:true}).click();await p.getByText('模板已保存',{exact:true}).waitFor()
  let current=await req(`${list}/${row.id}`,{actor:f.admin});assert.equal(current.enabled,false)
  await p.getByRole('checkbox').check();const restoring=p.waitForResponse(r=>r.request().method()==='PUT'&&new URL(r.url()).pathname===`${list}/${row.id}`);await p.getByRole('button',{name:'保存模板',exact:true}).click()
  await restoring
  current=await req(`${list}/${row.id}`,{actor:f.admin});assert.equal(current.enabled,true)
  await p.getByLabel('名称',{exact:false}).fill('Local unsaved')
  const backDialog=p.waitForEvent('dialog');await p.evaluate(()=>history.back());await(await backDialog).dismiss();await p.waitForURL(`**/admin/target-templates/${row.id}?created=1`)
  assert.equal(await p.getByLabel('名称',{exact:false}).inputValue(),'Local unsaved')
  p.once('dialog',d=>d.dismiss());await p.getByRole('link',{name:'靶机模板',exact:true}).last().click()
  assert.equal(await p.getByLabel('名称',{exact:false}).inputValue(),'Local unsaved')
  await save(row.id,{...payload,name:'Other administrator',lockVersion:current.lockVersion})
  await p.getByRole('button',{name:'保存模板',exact:true}).click();await p.getByRole('button',{name:'重新读取',exact:true}).waitFor()
  assert.equal(await p.getByLabel('名称',{exact:false}).inputValue(),'Local unsaved')
  assert.ok(await p.getByRole('button',{name:'保存模板',exact:true}).isDisabled())
  p.once('dialog',d=>d.accept());await p.getByRole('button',{name:'重新读取',exact:true}).click()
  await p.waitForFunction(()=>document.querySelector('input[maxlength="120"]')?.value==='Other administrator')
  await p.screenshot({path:'/tmp/attackbox-target-template-editor.png',fullPage:true})
  assert.equal(await p.getByRole('button',{name:'启动',exact:true}).count(),0)
  await p.getByRole('link',{name:'靶机模板',exact:true}).last().click()
  await p.getByRole('textbox',{name:'搜索模板'}).fill(prefix);await p.getByRole('button',{name:'搜索',exact:true}).click()
  await p.waitForFunction(()=>document.querySelectorAll('tbody tr').length===20)
  await p.getByRole('button',{name:'下一页',exact:true}).click();await p.waitForFunction(()=>document.querySelectorAll('tbody tr').length===1)
  assert.deepEqual(pageErrors,[])
 }finally{await browser?.close();for(const id of ids)await f.sql`delete from target_templates where id=${id}`;await f.cleanup()}
})
