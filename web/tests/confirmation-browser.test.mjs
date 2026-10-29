import test from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'

test(
  'themed confirmations protect navigation and deletion with focus, Escape and duplicate-submit handling',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 90000 },
  async () => {
    const f = await learningFixture()
    let browser
    try {
      const draft = await f.createModule('draft', { title: '确认交互验收' })
      browser = await chromium.launch({
        executablePath: '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      await context.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
      const page = await context.newPage(),
        native = [],
        errors = []
      page.setDefaultTimeout(6000)
      page.on('dialog', async dialog => {
        native.push(dialog.type())
        await dialog.dismiss()
      })
      page.on('pageerror', error => errors.push(error.message))
      await page.goto(f.base + '/learn')
      await page.goto(f.base + '/admin/modules')
      const remove = page.getByRole('button', { name: '删除', exact: true })
      const dialog = page.getByRole('alertdialog', { name: '永久删除模块？' })
      let deletes = 0,
        resolveDelete
      await page.route('**/api/admin/modules/' + draft.module.id, async route => {
        if (route.request().method() !== 'DELETE') return route.continue()
        deletes++
        await new Promise(resolve => {
          resolveDelete = resolve
        })
        await route.fulfill({
          status: 409,
          contentType: 'application/json',
          body: JSON.stringify({ message: '模块已改变，取消本次删除' })
        })
      })
      await remove.click()
      await dialog.waitFor()
      await page.waitForFunction(() => document.activeElement?.textContent === '取消')
      assert.equal(deletes, 0)
      await page.keyboard.press('Tab')
      assert.ok(await dialog.evaluate(el => el.contains(document.activeElement)))
      await page.mouse.click(10, 10)
      assert.equal(
        await dialog.isVisible(),
        true,
        'outside click must not accept a destructive action'
      )
      await page.keyboard.press('Escape')
      await dialog.waitFor({ state: 'hidden' })
      await page.waitForFunction(() => document.activeElement?.textContent === '删除')
      assert.equal(deletes, 0)
      for (const theme of ['midnight', 'graphite']) {
        await page.evaluate(theme => {
          document.documentElement.dataset.theme = theme
        }, theme)
        await remove.click()
        await dialog.waitFor()
        await page.screenshot({
          path: `/tmp/attackbox-confirm-${theme}.png`,
          fullPage: true,
          animations: 'disabled'
        })
        await dialog.getByRole('button', { name: '取消', exact: true }).click()
      }
      await page.setViewportSize({ width: 390, height: 844 })
      await remove.click()
      const box = await dialog.boundingBox()
      assert.ok(box.x >= 0 && box.x + box.width <= 390)
      await dialog.getByRole('button', { name: '取消', exact: true }).click()
      await page.setViewportSize({ width: 1440, height: 1000 })
      await remove.click()
      await page.goBack()
      await page.waitForURL('**/learn')
      assert.equal(await page.getByRole('alertdialog').count(), 0)
      assert.equal(deletes, 0, 'leaving the requester cancels its pending action')
      await page.goto(f.base + '/admin/modules')
      await remove.click()
      await dialog.getByRole('button', { name: '删除模块', exact: true }).click()
      await page.waitForFunction(() => document.querySelector('.delete-btn')?.disabled)
      assert.equal(await page.getByRole('alertdialog').count(), 0)
      assert.equal(await remove.isDisabled(), true)
      for (let n = 0; n < 50 && !resolveDelete; n++) await page.waitForTimeout(20)
      assert.equal(deletes, 1)
      resolveDelete()
      await page.getByRole('alert').filter({ hasText: '模块已改变' }).waitFor()
      assert.equal(await remove.isEnabled(), true)
      await page.goto(f.base + '/me?tab=profile')
      const nickname = page.getByLabel('昵称', { exact: true })
      await nickname.fill('未保存的昵称')
      await page
        .getByRole('navigation', { name: '个人中心' })
        .getByRole('link', { name: '通知', exact: true })
        .click()
      const leave = page.getByRole('alertdialog', { name: '放弃昵称修改？' })
      await leave.getByRole('button', { name: '继续编辑', exact: true }).click()
      assert.equal(new URL(page.url()).pathname, '/me')
      assert.equal(await nickname.inputValue(), '未保存的昵称')
      await page
        .getByRole('navigation', { name: '个人中心' })
        .getByRole('link', { name: '通知', exact: true })
        .click()
      await leave.getByRole('button', { name: '放弃修改', exact: true }).click()
      await page.waitForURL('**/notifications')
      assert.deepEqual(native, [])
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      await f.cleanup()
    }
  }
)

test(
  'stopping a learning target requires an explicit decision and cancellation sends no request',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 60000 },
  async () => {
    const f = await learningFixture()
    let browser
    try {
      const template = await f.seedTarget(),
        mod = await f.createModule()
      await f.sql`update modules set default_target_template_id=${template.id} where id=${mod.module.id}`
      browser = await chromium.launch({
        executablePath: '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const context = await browser.newContext()
      await context.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
      const page = await context.newPage()
      page.setDefaultTimeout(6000)
      let stops = 0
      await page.route('**/api/targets**', async route => {
        if (route.request().method() === 'POST') {
          assert.equal(route.request().postDataJSON().action, 'stop')
          stops++
          return route.fulfill({ json: { id: 'fixture-target', state: 'stopped' } })
        }
        return route.fulfill({
          json: {
            id: 'fixture-target',
            state: 'running',
            expiresAt: new Date(Date.now() + 3600000).toISOString(),
            instance: { ready: true, readiness: 'ready', ip: '10.201.0.2' }
          }
        })
      })
      await page.goto(`${f.base}/learn/modules/${mod.module.slug}/intro`)
      const stop = page.getByRole('button', { name: '停止靶机', exact: true })
      await stop.click()
      let dialog = page.getByRole('alertdialog', { name: '停止靶机？' })
      await dialog.getByRole('button', { name: '取消', exact: true }).click()
      assert.equal(stops, 0)
      await stop.click()
      await dialog.getByRole('button', { name: '停止靶机', exact: true }).click()
      await page.getByRole('button', { name: '启动靶机', exact: true }).waitFor()
      assert.equal(stops, 1)
    } finally {
      await browser?.close()
      await f.cleanup()
    }
  }
)
