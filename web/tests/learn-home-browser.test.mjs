import test from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { chromiumOptions } from './helpers/browser.mjs'
import { learningFixture } from './helpers/learning-fixture.mjs'
test(
  'learn homepage admin persistence, authorization, visibility, conflicts and carousel interaction',
  {
    skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.CONTENT_TEST_BASE_URL,
    timeout: 90000
  },
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
        {
          ...original.slides[0],
          id: 'first',
          title: '第一项测试',
          imageUrl: 'https://cdn.example.com/banner.svg',
          href: 'https://example.com/campaign?from=learn'
        },
        { ...original.slides[0], id: 'second', title: '第二项测试', imageUrl: '', href: '' },
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
      assert.equal(saved.slides[0].imageUrl, 'https://cdn.example.com/banner.svg')
      assert.equal(saved.slides[0].href, 'https://example.com/campaign?from=learn')
      await f.request(path, { actor: f.admin, method: 'PUT', body: config, status: 409 })
      assert.equal((await f.request('/api/learn-home')).data.slides.length, 2)
      browser = await chromium.launch(chromiumOptions())
      const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      await ctx.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
      const page = await ctx.newPage()
      await page.route('https://cdn.example.com/banner.svg', route =>
        route.fulfill({
          contentType: 'image/svg+xml',
          body: '<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="300"><rect width="1200" height="300" fill="#16435c"/></svg>'
        })
      )
      await page.goto(f.base + '/admin/learn-home')
      const title = page.locator('label').filter({ hasText: '标题 *' }).locator('input')
      await title.fill('首页推荐已更新')
      await page.getByRole('button', { name: '保存首页', exact: true }).click()
      await page.getByText('首页已更新', { exact: true }).waitFor()
      await page.reload()
      await title.waitFor()
      assert.equal(await title.inputValue(), '首页推荐已更新')
      await page.screenshot({ path: '/tmp/attackbox-learn-home-admin.png', fullPage: true })
      await page.clock.install()
      await page.goto(f.base + '/learn')
      const heading = page.locator('.feature-banner h1')
      await heading.waitFor()
      assert.equal(await heading.textContent(), '首页推荐已更新')
      await page.waitForFunction(() => !!document.getElementById('__nuxt')?.__vue_app__)
      await page.mouse.move(1400, 980)
      await page.clock.fastForward(5300)
      await page.getByRole('heading', { name: '第二项测试', exact: true }).waitFor()
      assert.equal(await heading.textContent(), '第二项测试')
      assert.equal(await page.locator('.feature-banner .banner-link').count(), 0)
      await page.locator('.feature-banner').hover()
      await page.clock.fastForward(5300)
      assert.equal(await heading.textContent(), '第二项测试')
      await page.getByRole('button', { name: '第 1 项：首页推荐已更新' }).click()
      assert.equal(await heading.textContent(), '首页推荐已更新')
      await page.screenshot({ path: '/tmp/attackbox-learn-home.png', fullPage: true })
      // Pointer selection must resume after leaving, despite the dot retaining DOM focus.
      await page.mouse.move(1400, 980)
      await page.clock.fastForward(5300)
      await page.getByRole('heading', { name: '第二项测试', exact: true }).waitFor()
      const fallback = page.locator('.banner-backdrop')
      await fallback.waitFor()
      const background = await fallback.getAttribute('style')
      assert.ok(background.includes('--banner-accent'))
      assert.equal(await fallback.locator('svg').count(), 1)
      // Keyboard focus intentionally pauses automatic content changes.
      await page.keyboard.press('Tab')
      await page.clock.fastForward(5300)
      assert.equal(await heading.textContent(), '第二项测试')
      await page.screenshot({ path: '/tmp/attackbox-carousel-fallback-light.png', fullPage: true })
      await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'midnight'))
      assert.equal(await fallback.getAttribute('style'), background)
      await page.screenshot({ path: '/tmp/attackbox-carousel-fallback-dark.png', fullPage: true })
      // Failed images use the same decoration path as omitted images.
      await page.route('https://cdn.example.com/banner.svg', route => route.abort())
      await page.reload()
      await page.locator('.feature-banner .banner-backdrop').waitFor()
      assert.equal(await page.locator('.feature-banner .banner-image').count(), 0)
      const failedBackground = await page.locator('.banner-backdrop').getAttribute('style')
      await page.reload()
      await page.locator('.feature-banner .banner-backdrop').waitFor()
      assert.equal(await page.locator('.banner-backdrop').getAttribute('style'), failedBackground)
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
