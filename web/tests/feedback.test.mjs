import test from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'

test(
  'feedback lifecycle: authenticated submission, admin handling, safe rendering and account-menu interaction',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 90000 },
  async () => {
    const f = await learningFixture(),
      authors = [f.admin.id]
    const ids = new Set()
    let browser
    const request = async (path, actor, method = 'GET', body, headers = {}) => {
      const response = await fetch(f.base + path, {
        method,
        headers: {
          ...(actor ? { cookie: `attackbox_session=${actor.token}` } : {}),
          ...(body ? { 'content-type': 'application/json' } : {}),
          ...headers
        },
        body: body ? JSON.stringify(body) : undefined
      })
      return { response, json: await response.json() }
    }
    const submit = async (actor, body) => {
      const result = await request('/api/me/feedback', actor, 'POST', body)
      if (result.json.data?.id) ids.add(result.json.data.id)
      return result
    }
    try {
      const learner = await f.account(),
        other = await f.account(),
        suspended = await f.account()
      authors.push(learner.id, other.id, suspended.id)
      const input = {
        category: 'bug',
        body: '环境启动按钮没有响应',
        sourcePath: '/learn?token=private#part',
        userId: other.id,
        status: 'resolved'
      }
      assert.equal((await submit(null, input)).response.status, 401)
      assert.equal((await request('/api/admin/feedback', learner)).response.status, 403)
      assert.equal((await request('/api/admin/feedback', null)).response.status, 401)
      assert.equal((await submit(learner, { ...input, category: 'invalid' })).response.status, 400)
      assert.equal((await submit(learner, { ...input, body: ' ' })).response.status, 400)
      assert.equal(
        (
          await request('/api/me/feedback', learner, 'POST', input, {
            origin: 'https://outside.test'
          })
        ).response.status,
        403
      )
      assert.equal(
        (
          await request(
            '/api/admin/feedback/not-an-id',
            f.admin,
            'PATCH',
            { status: 'resolved' },
            { origin: 'https://outside.test' }
          )
        ).response.status,
        403
      )
      const created = await submit(learner, input)
      assert.equal(created.response.status, 200)
      assert.match(created.response.headers.get('cache-control'), /no-store/)
      assert.ok(created.json.data.id)
      const id = created.json.data.id
      const listed = await request('/api/admin/feedback?page=1&status=new', f.admin)
      assert.equal(listed.response.status, 200)
      const stored = listed.json.data.items.find(item => item.id === id)
      assert.equal(stored.sourcePath, '/learn')
      assert.equal(stored.author.id, learner.id)
      assert.equal(stored.author.email, learner.email)
      assert.equal(stored.status, 'new')
      assert.equal(
        (await request(`/api/admin/feedback/${id}`, other, 'PATCH', { status: 'resolved' }))
          .response.status,
        403
      )
      assert.equal(
        (await request(`/api/admin/feedback/${id}`, f.admin, 'PATCH', { status: 'invalid' }))
          .response.status,
        400
      )
      assert.equal(
        (await request('/api/admin/feedback?status=invalid', f.admin)).response.status,
        400
      )
      assert.equal((await request('/api/admin/feedback?page=-1', f.admin)).response.status, 400)
      assert.equal(
        (
          await request(
            `/api/admin/feedback/00000000-0000-0000-0000-000000000000`,
            f.admin,
            'PATCH',
            { status: 'resolved' }
          )
        ).response.status,
        404
      )
      assert.equal(
        (await request(`/api/admin/feedback/${id}`, f.admin, 'PATCH', { status: 'in_progress' }))
          .response.status,
        200
      )
      assert.ok(
        (await request('/api/admin/feedback?status=in_progress', f.admin)).json.data.items.some(
          item => item.id === id
        )
      )
      assert.ok(
        !(await request('/api/admin/feedback?status=new', f.admin)).json.data.items.some(
          item => item.id === id
        )
      )

      // Serializing per-account submissions makes the small abuse limit reliable under concurrency.
      const concurrent = await Promise.all(
        Array.from({ length: 8 }, (_, n) => submit(other, { ...input, body: '并发反馈 ' + n }))
      )
      assert.equal(concurrent.filter(result => result.response.status === 200).length, 5)
      assert.equal(concurrent.filter(result => result.response.status === 429).length, 3)
      assert.equal(
        concurrent
          .find(result => result.response.status === 429)
          .response.headers.get('retry-after'),
        '600'
      )
      await f.sql`update feedback set created_at=now()-interval '11 minutes' where user_id=${other.id}`
      assert.equal((await submit(other, { ...input, body: '限流窗口已结束' })).response.status, 200)
      await f.sql`update users set status='suspended' where id=${suspended.id}`
      assert.equal((await submit(suspended, input)).response.status, 401)

      browser = await chromium.launch({
        executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      await context.addCookies([{ name: 'attackbox_session', value: learner.token, url: f.base }])
      const page = await context.newPage(),
        errors = []
      page.on('pageerror', error => errors.push(error.message))
      await page.goto(f.base + '/learn?token=private-context#part')
      const avatar = page.getByRole('button', { name: /用户菜单/ })
      const openFeedback = async () => {
        await avatar.click()
        await page.getByRole('button', { name: '问题反馈', exact: true }).click()
        const dialog = page.getByRole('dialog', { name: '问题反馈', exact: true })
        await dialog.waitFor()
        return dialog
      }
      let dialog = await openFeedback()
      const body = 'UI feedback: 建议优化这里\n<img src=x onerror="window.feedbackInjected=true">'
      await dialog.getByLabel('具体说说').fill(body)
      await dialog.getByText('功能建议', { exact: true }).click()
      assert.ok(!(await dialog.textContent()).includes('private-context'))
      await page.screenshot({ path: '/tmp/attackbox-feedback-composer.png' })
      await page.keyboard.press('Escape')
      await page.waitForFunction(() =>
        document.activeElement?.getAttribute('aria-label')?.startsWith('用户菜单')
      )
      dialog = await openFeedback()
      assert.equal(await dialog.getByLabel('具体说说').inputValue(), body)
      let release
      const gate = new Promise(resolve => {
        release = resolve
      })
      let posts = 0
      await page.route('**/api/me/feedback', async route => {
        posts++
        await gate
        await route.continue()
      })
      try {
        await dialog.getByRole('button', { name: '提交反馈', exact: true }).click()
        await page.waitForFunction(
          () => !!document.querySelector('.feedback-composer button[type=submit]:disabled')
        )
        await dialog.locator('form').dispatchEvent('submit')
        assert.equal(await dialog.getByRole('button', { name: '关闭反馈' }).isDisabled(), true)
      } finally {
        release()
      }
      await page.getByRole('heading', { name: '收到，谢谢你。' }).waitFor()
      assert.equal(posts, 1)
      await page.getByRole('button', { name: '继续探索', exact: true }).click()
      const [uiFeedback] =
        await f.sql`select * from feedback where user_id=${learner.id} and body=${body}`
      ids.add(uiFeedback.id)
      assert.equal(uiFeedback.category, 'suggestion')
      assert.equal(uiFeedback.source_path, '/learn')
      await context.close()

      const mobile = await browser.newContext({ viewport: { width: 390, height: 844 } })
      await mobile.addCookies([{ name: 'attackbox_session', value: learner.token, url: f.base }])
      await mobile.addInitScript(() => localStorage.setItem('attackbox-appearance', 'midnight'))
      const phone = await mobile.newPage()
      await phone.goto(f.base + '/learn')
      await phone.getByRole('button', { name: /用户菜单/ }).click()
      await phone.getByRole('button', { name: '问题反馈', exact: true }).click()
      const mobileDialog = phone.getByRole('dialog', { name: '问题反馈', exact: true })
      await mobileDialog.getByLabel('具体说说').fill('手机上的反馈同样可用。')
      const bounds = await mobileDialog.boundingBox()
      assert.ok(
        bounds.x >= 0 &&
          bounds.x + bounds.width <= 390 &&
          bounds.y >= 0 &&
          bounds.y + bounds.height <= 844
      )
      assert.equal(await phone.evaluate(() => document.activeElement?.tagName), 'TEXTAREA')
      await phone.screenshot({ path: '/tmp/attackbox-feedback-mobile.png' })
      await mobile.close()

      const admin = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      await admin.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
      const editor = await admin.newPage()
      editor.on('pageerror', error => errors.push(error.message))
      await editor.goto(f.base + '/admin/feedback')
      await editor.getByRole('heading', { name: '用户反馈', exact: true }).waitFor()
      await editor.getByRole('button').filter({ hasText: 'UI feedback:' }).click()
      const detail = editor.getByRole('article', { name: '反馈详情' })
      await detail.getByText(body, { exact: true }).waitFor()
      assert.equal(await editor.evaluate(() => window.feedbackInjected), undefined)
      await detail.getByText(learner.email, { exact: true }).waitFor()
      await detail.getByRole('button', { name: '处理中', exact: true }).click()
      await editor.getByRole('status').filter({ hasText: '已标记为处理中' }).waitFor()
      const filters = editor.getByRole('navigation', { name: '反馈筛选' })
      await filters.getByRole('button', { name: /^未处理/ }).click()
      await editor
        .getByRole('button')
        .filter({ hasText: 'UI feedback:' })
        .waitFor({ state: 'hidden' })
      await filters.getByRole('button', { name: /^处理中/ }).click()
      await editor.getByRole('button').filter({ hasText: 'UI feedback:' }).click()
      await detail.getByRole('button', { name: '已解决', exact: true }).click()
      await editor.getByRole('status').filter({ hasText: '已标记为已解决' }).waitFor()
      await filters.getByRole('button', { name: /^已解决/ }).click()
      await editor.getByRole('button').filter({ hasText: 'UI feedback:' }).click()
      await editor.evaluate(() => {
        localStorage.setItem('attackbox-appearance', 'midnight')
      })
      await editor.reload()
      await editor.getByRole('button').filter({ hasText: 'UI feedback:' }).click()
      await editor.screenshot({ path: '/tmp/attackbox-feedback-admin.png' })
      assert.equal(
        (await request('/api/admin/feedback?status=resolved', f.admin)).json.data.items.find(
          item => item.id === uiFeedback.id
        ).status,
        'resolved'
      )

      await editor.route('**/api/admin/feedback/*', route =>
        route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({ message: '暂时无法保存' })
        })
      )
      await detail.getByRole('button', { name: '未处理', exact: true }).click()
      await editor.getByRole('alert').filter({ hasText: '暂时无法保存' }).waitFor()
      assert.equal(
        await detail
          .getByRole('button', { name: '已解决', exact: true })
          .getAttribute('aria-pressed'),
        'true'
      )
      assert.deepEqual(errors, [])
      await admin.close()

      // Removing an account preserves the report without retaining a separate email snapshot.
      await f.sql`delete from users where id=${other.id}`
      const orphan = (await request('/api/admin/feedback', f.admin)).json.data.items.find(
        item => item.author.id === null
      )
      assert.equal(orphan.author.name, '已注销用户')
      assert.equal(orphan.author.email, null)
    } finally {
      if (browser) await browser.close()
      for (const id of ids) await f.sql`delete from feedback where id=${id}`
      for (const id of authors) await f.sql`delete from feedback where user_id=${id}`
      await f.cleanup()
    }
  }
)
