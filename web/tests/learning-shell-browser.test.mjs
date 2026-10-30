import test from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { chromiumOptions } from './helpers/browser.mjs'
import { learningFixture } from './helpers/learning-fixture.mjs'
test(
  'shared content alignment, empty-category filtering, lazy navigation, cached return and discussion placeholder',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 90000 },
  async () => {
    const f = await learningFixture()
    let browser, release
    try {
      const learner = await f.account()
      assert.equal((await f.sql`select id from content_categories`).length, 0)
      const [category] =
        await f.sql`insert into content_categories (slug,name) values ('web-security','Web 安全') returning id`
      await f.createModule('publish', { title: '对齐与导航验收', categoryId: category.id })
      browser = await chromium.launch(chromiumOptions())
      const context = await browser.newContext({ viewport: { width: 1920, height: 1080 } })
      await context.addCookies([{ name: 'attackbox_session', value: learner.token, url: f.base }])
      const page = await context.newPage()
      const errors = []
      page.on('pageerror', e => errors.push(e.message))
      let moduleRequests = 0
      page.on('request', request => {
        if (new URL(request.url()).pathname === '/api/modules') moduleRequests++
      })
      const nav = () => page.getByRole('navigation', { name: '主导航' })
      const aligned = async selector => {
        const frame = await page.locator('.learning-shell').boundingBox()
        const header = await page.locator('.header-inner').boundingBox()
        const box = await page.locator(selector).boundingBox()
        const width = await page.evaluate(() => document.documentElement.clientWidth)
        assert.ok(Math.abs(frame.x - (width - frame.width) / 2) < 1, 'frame is centered')
        assert.ok(frame.width <= 1360 && frame.width <= width - 64, 'bounded desktop width')
        assert.ok(
          Math.abs(header.x - frame.x) < 1 && Math.abs(header.width - frame.width) < 1,
          'header shares frame edges'
        )
        assert.ok(
          Math.abs(box.x - frame.x) < 1 && Math.abs(box.width - frame.width) < 1,
          `${selector} fills the content frame`
        )
        const brand = await page.locator('.learning-header .brand').boundingBox()
        assert.ok(Math.abs(brand.x - frame.x) < 1, 'content aligns with brand boundary')
        assert.ok(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          'no horizontal overflow'
        )
      }
      await page.goto(f.base + '/discuss')
      await page.getByRole('heading', { name: '好问题，值得聊聊。' }).waitFor()
      await aligned('.discuss-card')
      await nav().getByRole('link', { name: '学习', exact: true }).click()
      await page.getByRole('heading', { name: '对齐与导航验收' }).waitFor()
      await aligned('.feature-banner')
      assert.equal(await page.locator('.filters .filter-btn').count(), 2)
      assert.equal(
        await page
          .locator('.filters')
          .getByRole('button', { name: 'Linux 基础', exact: true })
          .count(),
        0
      )
      assert.equal(moduleRequests, 1)
      await page.screenshot({ path: '/tmp/attackbox-learning-alignment.png', fullPage: true })
      const blocked = new Promise(resolve => {
        release = resolve
      })
      await page.route('**/api/challenges', async route => {
        await blocked
        await route.continue()
      })
      await nav().getByRole('link', { name: '挑战', exact: true }).click()
      await page.waitForURL('**/challenges', { timeout: 3000 })
      await page.getByRole('status', { name: '正在读取挑战' }).waitFor()
      await aligned('.page-head')
      release()
      await page.getByText('暂无符合条件的挑战。', { exact: true }).waitFor()
      await nav().getByRole('link', { name: '学习', exact: true }).click()
      await page.getByRole('heading', { name: '对齐与导航验收' }).waitFor()
      assert.equal(moduleRequests, 1, 'returning within the short cache must not refetch modules')
      assert.equal(await page.getByRole('status', { name: '正在读取学习模块' }).count(), 0)
      await page.clock.install()
      await page.clock.fastForward(46_000)
      await nav().getByRole('link', { name: '挑战', exact: true }).click()
      await page.waitForURL('**/challenges')
      await Promise.all([
        page.waitForRequest(r => new URL(r.url()).pathname === '/api/modules'),
        nav().getByRole('link', { name: '学习', exact: true }).click()
      ])
      assert.equal(moduleRequests, 2, 'expired catalogue cache must fetch fresh data')
      await page.getByRole('button', { name: /用户菜单/ }).click()
      await page.getByRole('link', { name: '个人中心', exact: true }).click()
      await page.locator('.personal-sidebar').waitFor()
      await aligned('.personal-center')
      await page.screenshot({ path: '/tmp/attackbox-personal-alignment.png', fullPage: true })
      await nav().getByRole('link', { name: '讨论', exact: true }).click()
      await page.getByRole('heading', { name: '好问题，值得聊聊。' }).waitFor()
      await page.screenshot({ path: '/tmp/attackbox-discuss-placeholder.png', fullPage: true })
      // Catalog pages use identical edges at common desktop sizes, including ultrawide.
      for (const width of [1280, 1440, 1920, 2560]) {
        await page.setViewportSize({ width, height: 1080 })
        for (const [path, selector] of [
          ['/learn', '.page-tabs'],
          ['/challenges', '.page-head'],
          ['/references', '.references-index > header'],
          ['/discuss', '.discuss-card']
        ]) {
          await page.goto(f.base + path)
          await page.locator(selector).waitFor()
          await aligned(selector)
          if (width === 1920 || width === 2560)
            await page.screenshot({ path: `/tmp/attackbox-centered-${path.slice(1)}-${width}.png` })
        }
      }
      await page.setViewportSize({ width: 390, height: 844 })
      await page.waitForTimeout(100)
      assert.ok(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
        'mobile layout must not overflow'
      )
      await nav().getByRole('link', { name: '学习', exact: true }).click()
      await page.getByRole('heading', { name: '对齐与导航验收' }).waitFor()
      assert.ok(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
        'learning page must fit mobile'
      )
      await page.getByRole('button', { name: /用户菜单/ }).click()
      await page.getByRole('link', { name: '个人中心', exact: true }).click()
      await page.locator('.personal-sidebar').waitFor()
      assert.ok(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
        'personal center must fit mobile'
      )
      assert.deepEqual(errors, [])
    } finally {
      release?.()
      await browser?.close()
      await f.sql`delete from modules where category_id in (select id from content_categories where slug='web-security')`
      await f.sql`delete from content_categories where slug='web-security'`
      await f.cleanup()
    }
  }
)
