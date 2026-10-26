import test from 'node:test'
import assert from 'node:assert/strict'
import { learningFixture } from './helpers/learning-fixture.mjs'
import { chromium } from 'playwright'
import { existsSync } from 'node:fs'
test(
  'Shared account menu, appearance, profile and administrator announcements',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 60000 },
  async () => {
    const f = await learningFixture()
    let browser
    const json = async (path, actor = f.admin, method = 'GET', body) => {
      const r = await fetch(f.base + path, {
        method,
        headers: {
          ...(actor ? { cookie: `attackbox_session=${actor.token}` } : {}),
          ...(body ? { 'content-type': 'application/json' } : {}),
        },
        body: body ? JSON.stringify(body) : undefined,
      })
      return { r, data: await r.json() }
    }
    try {
      const a = await f.account(),
        b = await f.account()
      assert.equal((await json('/api/me/notifications', null)).r.status, 401)
      assert.equal(
        (
          await json('/api/admin/notifications', a, 'POST', {
            title: 'x',
            body: 'y',
          })
        ).r.status,
        403,
      )
      for (const link of [
        'https://outside.test',
        '//outside.test',
        '/\\outside.test',
      ])
        assert.equal(
          (
            await json('/api/admin/notifications', f.admin, 'POST', {
              title: 'x',
              body: 'y',
              link,
            })
          ).r.status,
          400,
        )
      const csrf = await fetch(f.base + '/api/admin/notifications', {
        method: 'POST',
        headers: {
          cookie: `attackbox_session=${f.admin.token}`,
          origin: 'https://outside.test',
          'content-type': 'application/json',
        },
        body: JSON.stringify({ title: 'x', body: 'y' }),
      })
      assert.equal(csrf.status, 403)
      browser = await chromium.launch({
        executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH || (existsSync('/usr/bin/chromium') ? '/usr/bin/chromium' : undefined),
        args: ['--no-sandbox'],
      })
      const admin = await browser.newContext({
        viewport: { width: 1440, height: 1000 },
      })
      await admin.addCookies([
        { name: 'attackbox_session', value: f.admin.token, url: f.base },
      ])
      const editor = await admin.newPage(),
        errors = []
      editor.on('pageerror', (e) => errors.push(e.message))
      await editor.goto(f.base + '/admin/notifications')
      await editor.getByLabel('标题', { exact: true }).fill('课程更新通知')
      await editor
        .getByLabel('正文', { exact: true })
        .fill(
          '新课程已上线。\n<script>window.notificationInjected=true</script>',
        )
      await editor.getByLabel('相关页面', { exact: false }).fill('/learn')
      await editor
        .getByRole('button', { name: '发布通知', exact: true })
        .click()
      await editor.getByText('通知已发布', { exact: true }).waitFor()
      const list = await json('/api/me/notifications', a)
      assert.equal(list.data.data.unread, 1)
      assert.match(list.r.headers.get('cache-control'), /no-store/)
      const id = list.data.data.items[0].id
      const viewer = await browser.newContext({
        viewport: { width: 1440, height: 1000 },
      })
      await viewer.addCookies([
        { name: 'attackbox_session', value: a.token, url: f.base },
      ])
      const page = await viewer.newPage()
      page.on('pageerror', (e) => errors.push(e.message))
      await page.goto(f.base + '/learn')
      await page.getByRole('button', { name: /用户菜单/ }).click()
      const popup = page.getByRole('dialog', { name: '用户菜单' })
      await popup.getByRole('link', { name: /通知/ }).waitFor()
      const panelStyle = await popup.evaluate((e) => ({
        background: getComputedStyle(e).backgroundColor,
        radius: getComputedStyle(e).borderRadius,
        width: e.getBoundingClientRect().width,
      }))
      assert.notEqual(panelStyle.background, 'rgba(0, 0, 0, 0)')
      assert.equal(panelStyle.radius, '16px')
      assert.ok(panelStyle.width >= 250 && panelStyle.width < 310)
      await page
        .locator('.account-count')
        .getByText('1', { exact: true })
        .waitFor()
      await popup.getByRole('radio', { name: '深海', exact: true }).check()
      assert.equal(
        await page.locator('html').getAttribute('data-theme'),
        'midnight',
      )
      await page.keyboard.press('Escape')
      await popup.waitFor({ state: 'hidden' })
      assert.match(
        await page.evaluate(() =>
          document.activeElement?.getAttribute('aria-label'),
        ),
        /用户菜单/,
      )
      await page.reload()
      await page.getByRole('button', { name: /用户菜单/ }).click()
      await page.getByRole('radio', { name: '深海', exact: true }).waitFor()
      assert.equal(
        await page
          .getByRole('radio', { name: '深海', exact: true })
          .isChecked(),
        true,
      )
      await page.getByRole('radio', { name: '石墨', exact: true }).check()
      await page.screenshot({ path: '/tmp/attackbox-user-menu.png', animations: 'disabled' })
      await page.mouse.click(100, 300)
      await popup.waitFor({ state: 'hidden' })
      await page.getByRole('button', { name: /用户菜单/ }).click()
      await popup.getByRole('link', { name: /通知/ }).click()
      await page.waitForURL('**/notifications')
      await page.getByRole('button', { name: /课程更新通知/ }).click()
      await page
        .locator('article')
        .getByText('<script>window.notificationInjected=true</script>', {
          exact: false,
        })
        .waitFor()
      assert.equal(await page.locator('article script').count(), 0)
      assert.equal(
        (await json('/api/me/notifications/unread', a)).data.count,
        0,
      )
      assert.equal(
        (await json('/api/me/notifications/unread', b)).data.count,
        1,
      )
      await json(`/api/me/notifications/${id}/read`, a, 'POST', {
        userId: b.id,
      })
      assert.equal(
        (await json('/api/me/notifications/unread', b)).data.count,
        1,
      )
      assert.equal(
        (
          await f.sql`select count(*)::int as n from notification_reads where user_id=${a.id}`
        )[0].n,
        1,
      )
      await page.goto(f.base + '/me')
      await page.getByLabel('昵称', { exact: true }).fill('Learning Reader')
      await page.getByRole('button', { name: '保存资料', exact: true }).click()
      await page.getByText('资料已保存', { exact: true }).waitFor()
      assert.equal(
        (await json('/api/auth/me', a)).data.user.displayName,
        'Learning Reader',
      )
      await json('/api/me/profile', a, 'PUT', {
        displayName: 'Reader',
        role: 'admin',
        email: 'spoof@test.test',
      })
      const me = (await json('/api/auth/me', a)).data.user
      assert.equal(me.role, 'user')
      assert.equal(me.email, a.email)
      await page.goto(f.base + '/')
      await page.getByRole('button', { name: /用户菜单/ }).click()
      await page
        .getByRole('dialog', { name: '用户菜单' })
        .getByRole('link', { name: '个人中心', exact: true })
        .waitFor()
      await page.keyboard.press('Escape')
      await page.goto(f.base + '/learn')
      await page.getByRole('button', { name: /用户菜单/ }).click()
      await page.route('**/api/auth/logout', (r) =>
        r.fulfill({ status: 503, contentType: 'application/json', body: '{}' }),
      )
      await page.getByRole('button', { name: '退出登录', exact: true }).click()
      await page.getByText('退出未确认，请重试', { exact: true }).waitFor()
      await page
        .getByRole('dialog', { name: '用户菜单' })
        .getByRole('link', { name: '个人中心', exact: true })
        .waitFor()
      await page.unroute('**/api/auth/logout')
      await page.getByRole('button', { name: '退出登录', exact: true }).click()
      await page
        .getByRole('dialog', { name: '用户菜单' })
        .waitFor({ state: 'hidden' })
      await viewer.addCookies([
        { name: 'attackbox_session', value: b.token, url: f.base },
      ])
      await page.reload()
      await page.getByRole('button', { name: /用户菜单/ }).click()
      await page.getByText(b.email, { exact: true }).waitFor()
      assert.equal(await page.getByText(a.email, { exact: true }).count(), 0)
      await page.setViewportSize({ width: 390, height: 844 })
      assert.ok(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      )
      await page.keyboard.press('Escape')
      editor.once('dialog', (d) => d.accept())
      await editor.getByRole('button', { name: '撤回', exact: true }).click()
      await editor.getByText('通知已撤回', { exact: true }).waitFor()
      assert.equal(
        (await json('/api/me/notifications', b)).data.data.items.length,
        0,
      )
      assert.equal(
        (await json('/api/me/notifications/unread', b)).data.count,
        0,
      )
      assert.equal(
        (await json(`/api/me/notifications/${id}/read`, b, 'POST', {})).r
          .status,
        404,
      )
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      await f.sql`delete from announcements where author_id=${f.admin.id}`
      await f.cleanup()
    }
  },
)
