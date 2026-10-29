import test from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'
test(
  'learn homepage admin persistence, authorization, visibility, conflicts and carousel interaction',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 90000 },
  async () => {
    const f = await learningFixture()
    let browser
    try {
      const path = '/api/admin/learn-home',
        user = await f.account()
      const original = (await f.request(path, { actor: f.admin })).data
      await f.request(path, { status: 401 })
      await f.request(path, { actor: user, status: 403 })
      await f.request(path, { actor: user, method: 'PUT', body: original, status: 403 })
      const slides = [
        { ...original.slides[0], id: 'first', title: '第一项测试', href: '/learn' },
        { ...original.slides[0], id: 'second', title: '第二项测试', href: '/learn' },
        { ...original.slides[0], id: 'hidden', title: '隐藏项', enabled: false }
      ]
      const config = { ...original, slides, autoplay: true, intervalSeconds: 5 }
      await f.request(path, {
        actor: f.admin,
        method: 'PUT',
        body: { ...config, intervalSeconds: 1 },
        status: 400
      })
      const saved = (await f.request(path, { actor: f.admin, method: 'PUT', body: config })).data
      assert.equal(saved.lockVersion, 1)
      await f.request(path, { actor: f.admin, method: 'PUT', body: config, status: 409 })
      assert.equal((await f.request('/api/learn-home')).data.slides.length, 2)
      browser = await chromium.launch({
        executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      await ctx.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
      const page = await ctx.newPage()
      await page.goto(f.base + '/admin/learn-home')
      const title = page.locator('label').filter({ hasText: '标题 *' }).locator('input')
      await title.fill('首页推荐已更新')
      await page.getByRole('button', { name: '保存首页', exact: true }).click()
      await page.getByText('首页已更新', { exact: true }).waitFor()
      await page.reload()
      await title.waitFor()
      assert.equal(await title.inputValue(), '首页推荐已更新')
      await page.screenshot({ path: '/tmp/attackbox-learn-home-admin.png', fullPage: true })
      await page.goto(f.base + '/learn')
      const heading = page.locator('.feature-banner h1')
      await heading.waitFor()
      assert.equal(await heading.textContent(), '首页推荐已更新')
      await page.waitForTimeout(5300)
      assert.equal(await heading.textContent(), '第二项测试')
      await page.locator('.feature-banner').hover()
      await page.waitForTimeout(5300)
      assert.equal(await heading.textContent(), '第二项测试')
      await page.getByRole('button', { name: '第 1 项：首页推荐已更新' }).click()
      assert.equal(await heading.textContent(), '首页推荐已更新')
      await page.screenshot({ path: '/tmp/attackbox-learn-home.png', fullPage: true })
      const latest = (await f.request(path, { actor: f.admin })).data
      await f.request(path, { actor: f.admin, method: 'PUT', body: { ...latest, enabled: false } })
      await page.reload()
      await page.locator('.page-tabs').waitFor()
      assert.equal(await page.locator('.feature-banner').count(), 0)
    } finally {
      await browser?.close()
      await f.sql`DELETE FROM learn_home_settings`
      await f.cleanup()
    }
  }
)
