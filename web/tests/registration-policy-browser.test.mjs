import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'
test('registration rejects temporary domains before account creation and rechecks pending activation', {skip:!process.env.CONTENT_TEST_DATABASE_URL,timeout:60000},async()=>{
 const f=await learningFixture();let browser
 try{
  const publicPolicy=await fetch(f.base+'/api/auth/registration-policy')
  assert.equal(publicPolicy.status,200);assert.equal(publicPolicy.headers.get('cache-control'),'private, no-store')
  const {domains}=await publicPolicy.json();assert.ok(domains.includes('gmail.com'));assert.ok(!domains.includes('mailinator.com'))
  for(const domain of ['mailinator.com','gmail.com.evil.test','sub.gmail.com']){
   const email=`blocked-${crypto.randomUUID()}@${domain}`
   await f.request('/api/auth/register',{method:'POST',body:{email,password:'password-for-local-test'},status:403})
   assert.equal((await f.sql`select id from users where email=${email}`).length,0)
  }
  const legacy=await f.account()
  assert.ok((await f.request('/api/auth/login',{method:'POST',body:{email:legacy.email,password:legacy.password}})).response.ok)
  const pending=await f.account(),token=crypto.randomBytes(32).toString('hex')
  await f.sql`update users set status='pending',email_verified_at=null where id=${pending.id}`
  await f.sql`insert into email_tokens(user_id,target_email,purpose,token_hash,expires_at) values(${pending.id},${pending.email},'verify_email',${crypto.createHash('sha256').update(token).digest()},now()+interval '1 hour')`
  await f.request('/api/auth/verify-email',{method:'POST',body:{token},status:403})
  assert.equal((await f.sql`select status from users where id=${pending.id}`)[0].status,'pending')
  assert.equal((await f.sql`select consumed_at from email_tokens where user_id=${pending.id}`)[0].consumed_at,null)
  await f.request('/api/auth/resend-verification',{method:'POST',body:{email:pending.email}})
  assert.equal((await f.sql`select count(*)::int as n from email_tokens where user_id=${pending.id}`)[0].n,1)
  browser=await chromium.launch({executablePath:'/usr/bin/chromium',args:['--no-sandbox']})
  const page=await browser.newPage();await page.goto(f.base+'/auth?mode=register')
  await page.locator('#email:enabled').fill('blocked@mailinator.com');await page.locator('#password').fill('local-test-password')
  await page.getByRole('button',{name:'继续创建账号',exact:true}).click()
  await page.getByText('该邮箱域名暂不支持注册。',{exact:true}).waitFor()
  await page.screenshot({path:'/tmp/attackbox-registration-domains.png',animations:'disabled'})
 }finally{await browser?.close();await f.cleanup()}
})
