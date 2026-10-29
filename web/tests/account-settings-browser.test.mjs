import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'
test(
  'account security, sender identity, personal deletion, private recipients and shared account UI',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 120000 },
  async () => {
    const f = await learningFixture()
    let browser, a, b, c
    try {
      a = await f.account()
      b = await f.account()
      c = await f.account()
      const newPassword = 'new-account-password-2026'
      await f.request('/api/me/profile', { status: 401 })
      assert.deepEqual((await f.request('/api/me/profile', { actor: a })).data, {
        id: a.id,
        hasPassword: true
      })
      await f.request('/api/me/password', {
        method: 'PUT',
        body: { currentPassword: a.password, newPassword },
        status: 401
      })
      await f.request('/api/me/password', {
        actor: a,
        method: 'PUT',
        body: { currentPassword: a.password, newPassword: 'short' },
        status: 400
      })
      await f.request('/api/me/password', {
        actor: b,
        method: 'PUT',
        body: { userId: a.id, currentPassword: a.password, newPassword },
        status: 400
      })
      const cross = await fetch(f.base + '/api/me/password', {
        method: 'PUT',
        headers: {
          cookie: `attackbox_session=${a.token}`,
          origin: 'https://elsewhere.example',
          'content-type': 'application/json'
        },
        body: JSON.stringify({ currentPassword: a.password, newPassword })
      })
      assert.equal(cross.status, 403)
      const otherToken = crypto.randomBytes(32).toString('hex'),
        reset = crypto.randomBytes(32).toString('hex')
      await f.sql`insert into auth_sessions(user_id,token_hash,expires_at) values(${a.id},${crypto.createHash('sha256').update(otherToken).digest()},now()+interval '1 hour')`
      await f.sql`insert into email_tokens(user_id,purpose,target_email,token_hash,expires_at) values(${a.id},'reset_password',${a.email},${crypto.createHash('sha256').update(reset).digest()},now()+interval '30 minutes')`
      browser = await chromium.launch({
        executablePath: '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      await ctx.addCookies([{ name: 'attackbox_session', value: a.token, url: f.base }])
      const page = await ctx.newPage(),
        errors = []
      page.on('pageerror', e => errors.push(e.message))
      await page.goto(f.base + '/me?tab=profile')
      await page.getByRole('button', { name: '修改密码', exact: true }).click()
      await page.getByLabel('当前密码', { exact: true }).waitFor()
      assert.ok(
        !/\bACCOUNT\b|\bCHALLENGES\b|\bLEARNING\b|个人中心/.test(
          await page.locator('body').innerText()
        )
      )
      await page.getByLabel('当前密码', { exact: true }).fill('wrong-current-password')
      await page.getByLabel('新密码', { exact: true }).fill(newPassword)
      await page.getByLabel('确认新密码', { exact: true }).fill('different-new-password')
      await page.getByRole('button', { name: '更新密码', exact: true }).click()
      await page.getByText('两次新密码不一致', { exact: true }).waitFor()
      await page.getByLabel('确认新密码', { exact: true }).fill(newPassword)
      await page.getByRole('button', { name: '更新密码', exact: true }).click()
      await page.getByText('当前密码不正确', { exact: true }).waitFor()
      await page.getByLabel('当前密码', { exact: true }).fill(a.password)
      await page.getByRole('button', { name: '更新密码', exact: true }).click()
      await page.getByText('密码已更新，其他设备需重新登录。', { exact: true }).waitFor()
      assert.equal(await page.getByLabel('新密码', { exact: true }).inputValue(), '')
      await f.request('/api/auth/me', { actor: a })
      await f.request('/api/auth/me', { actor: { token: otherToken }, status: 401 })
      assert.ok(
        (
          await f.sql`select consumed_at from email_tokens where user_id=${a.id} and purpose='reset_password'`
        )[0].consumed_at
      )
      await f.request('/api/auth/login', {
        method: 'POST',
        body: { email: a.email, password: a.password },
        status: 401
      })
      await f.request('/api/auth/login', {
        method: 'POST',
        body: { email: a.email, password: newPassword }
      })
      await page.screenshot({ path: '/tmp/attackbox-account-profile-graphite.png', fullPage: true })
      const variants = ['concurrent-new-password-one', 'concurrent-new-password-two']
      const results = await Promise.all(
        variants.map(password =>
          fetch(f.base + '/api/me/password', {
            method: 'PUT',
            headers: { cookie: `attackbox_session=${a.token}`, 'content-type': 'application/json' },
            body: JSON.stringify({ currentPassword: newPassword, newPassword: password })
          })
        )
      )
      assert.equal(results.filter(r => r.status === 200).length, 1)
      assert.ok(results.every(r => [200, 400, 409].includes(r.status)))
      const winner = variants[results.findIndex(r => r.status === 200)]
      await f.request('/api/auth/login', {
        method: 'POST',
        body: { email: a.email, password: winner }
      })
      await f.request('/api/me/profile', {
        actor: a,
        method: 'PUT',
        body: { displayName: '赤色小雀' }
      })
      await f.sql`update users set display_name='站务小红' where id=${f.admin.id}`
      const first = (
        await f.request('/api/admin/notifications', {
          actor: f.admin,
          method: 'POST',
          body: { title: '第一条平台消息', body: '公开给本站用户的平台通知。' }
        })
      ).data.id
      await f.request('/api/admin/notifications', {
        actor: f.admin,
        method: 'POST',
        body: { title: '课程更新通知', body: '从小问题开始,试出新思路。' }
      })
      const [privateMessage] =
        await f.sql`insert into notifications(kind,sender_id,recipient_id,title,body) values('message',${a.id},${b.id},'会员消息样例','这是一条仅发给指定接收人的消息。') returning id`
      const list = (await f.request('/api/me/notifications', { actor: a })).data
      assert.equal(list.items.length, 2)
      assert.equal(list.items[0].sender.name, '站务小红')
      assert.equal(
        (await f.request('/api/admin/notifications', { actor: f.admin })).data.items.length,
        2,
        'administrator history must not expose member messages'
      )
      await f.request(`/api/me/notifications/${privateMessage.id}/read`, {
        actor: c,
        method: 'POST',
        body: {},
        status: 404
      })
      await f.request(`/api/me/notifications/${privateMessage.id}`, {
        actor: c,
        method: 'DELETE',
        status: 404
      })
      await f.request(`/api/admin/notifications/${privateMessage.id}/withdraw`, {
        actor: f.admin,
        method: 'POST',
        body: {},
        status: 404
      })
      const deletion = await fetch(f.base + `/api/me/notifications/${first}`, {
        method: 'DELETE',
        headers: { cookie: `attackbox_session=${a.token}` }
      })
      assert.equal(deletion.status, 200)
      const deleted = await deletion.json()
      assert.equal(deleted.unread, 1)
      await f.request(`/api/me/notifications/${first}`, { actor: a, method: 'DELETE' })
      await f.request(`/api/me/notifications/${first}/read`, {
        actor: a,
        method: 'POST',
        body: {},
        status: 404
      })
      assert.equal((await f.request('/api/me/notifications', { actor: b })).data.items.length, 3)
      await page.goto(f.base + '/notifications')
      await page.getByRole('button', { name: /课程更新通知/ }).click()
      await page.locator('.notification-sender').getByText('站务小红', { exact: true }).waitFor()
      await page.screenshot({
        path: '/tmp/attackbox-account-notifications-graphite.png',
        fullPage: true
      })
      await page.evaluate(() => {
        localStorage.setItem('attackbox-appearance', 'midnight')
      })
      await page.reload()
      await page.getByRole('button', { name: /课程更新通知/ }).click()
      await page.screenshot({
        path: '/tmp/attackbox-account-notifications-midnight.png',
        fullPage: true
      })
      await page.getByRole('button', { name: '删除消息', exact: true }).click()
      await page.getByRole('heading', { name: '暂无通知', exact: true }).waitFor()
      assert.equal((await f.request('/api/me/notifications', { actor: a })).data.items.length, 0)
      assert.equal((await f.request('/api/me/notifications', { actor: b })).data.items.length, 3)
      assert.equal(
        (await f.request('/api/admin/notifications', { actor: f.admin })).data.items.length,
        2
      )
      const peer = await browser.newContext({ viewport: { width: 390, height: 844 } })
      await peer.addCookies([{ name: 'attackbox_session', value: b.token, url: f.base }])
      const mobile = await peer.newPage()
      await mobile.goto(f.base + '/notifications')
      await mobile.getByRole('button', { name: /会员消息样例/ }).click()
      await mobile.locator('.notification-sender').getByText('赤色小雀', { exact: true }).waitFor()
      assert.ok(await mobile.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      await mobile.screenshot({
        path: '/tmp/attackbox-account-notifications-mobile.png',
        fullPage: true
      })
      assert.equal(await mobile.locator('.notification-list').isVisible(), false)
      await mobile.getByRole('button', { name: '全部消息', exact: true }).click()
      await mobile.getByRole('button', { name: /会员消息样例/ }).waitFor()
      await mobile.goto(f.base + '/me?tab=profile')
      await mobile.getByRole('button', { name: '修改密码', exact: true }).click()
      await mobile.getByLabel('当前密码', { exact: true }).waitFor()
      assert.ok(await mobile.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      await mobile.screenshot({ path: '/tmp/attackbox-account-profile-mobile.png', fullPage: true })
      await f.sql`update users set password_hash=null where id=${c.id}`
      assert.deepEqual((await f.request('/api/me/profile', { actor: c })).data, {
        id: c.id,
        hasPassword: false
      })
      await f.request('/api/me/password', {
        actor: c,
        method: 'PUT',
        body: { currentPassword: 'anything', newPassword: 'new-password-for-passwordless' },
        status: 409
      })
      const passwordless = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      await passwordless.addCookies([{ name: 'attackbox_session', value: c.token, url: f.base }])
      const noPasswordPage = await passwordless.newPage()
      await noPasswordPage.goto(f.base + '/me?tab=profile')
      await noPasswordPage.getByText('未设置', { exact: true }).waitFor()
      assert.equal(
        await noPasswordPage.getByRole('button', { name: '修改密码', exact: true }).count(),
        0
      )
      assert.equal(await noPasswordPage.locator('input[type=password]').count(), 0)
      assert.equal(
        (await f.sql`select password_hash from users where id=${c.id}`)[0].password_hash,
        null
      )
      await noPasswordPage.screenshot({
        path: '/tmp/attackbox-profile-passwordless.png',
        animations: 'disabled'
      })
      await mobile.getByRole('button', { name: '取消修改密码', exact: true }).click()
      await mobile.getByRole('button', { name: '修改密码', exact: true }).click()
      assert.equal(await mobile.getByLabel('当前密码', { exact: true }).inputValue(), '')
      await mobile.getByRole('button', { name: '取消修改密码', exact: true }).click()
      await mobile.screenshot({
        path: '/tmp/attackbox-profile-unified.png',
        fullPage: true,
        animations: 'disabled'
      })
      await mobile.goto(f.base + '/auth')
      await mobile.locator('#email:enabled').waitFor()
      assert.equal(await mobile.locator('[data-provider]').count(), 0)
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      await f.sql`delete from notifications where sender_id in (${f.admin.id},${a?.id || f.admin.id})`
      await f.cleanup()
    }
  }
)
