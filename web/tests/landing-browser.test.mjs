import test from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'

test(
  'landing renders live public catalog in SSR and lets visitors browse without login',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 90000 },
  async () => {
    const f = await learningFixture()
    let browser
    try {
      assert.equal(
        (await f.sql`select count(*)::int as n from modules`)[0].n,
        0,
        'use an empty isolated database'
      )
      let html = await (await fetch(f.base + '/')).text()
      assert.ok(html.includes('新的内容，即将开始。'))
      assert.ok(!html.includes('云原生攻防'))
      assert.ok(html.includes('href="/learn"'))
      const published = []
      for (let index = 0; index < 7; index++) {
        const mod = await f.createModule('publish', {
          title: `真实模块${index}`,
          summary: `数据库简介${index}`,
          chapters: [
            {
              title: '公开章节',
              slug: 'public',
              access: 'public',
              bodyMarkdown: '公开内容',
              durationMinutes: 12,
              questions: []
            },
            {
              title: '登录章节',
              slug: 'private',
              access: 'authenticated',
              bodyMarkdown: '不应出现在首页的私密正文',
              durationMinutes: 18,
              questions: [{ promptMarkdown: '问题', answer: 'secret-answer' }]
            }
          ]
        })
        published.push(mod)
        await f.sql`update modules set published_at = now() + ${index} * interval '1 minute' where id=${mod.module.id}`
      }
      await f.createModule('draft', { title: '不应公开的草稿' })
      const response = await f.request('/api/landing')
      assert.equal(response.data.length, 6)
      assert.deepEqual(
        response.data.map(m => m.title),
        [6, 5, 4, 3, 2, 1].map(i => `真实模块${i}`)
      )
      assert.equal(response.data[0].chapterCount, 2)
      assert.equal(response.data[0].durationMinutes, 30)
      assert.equal(response.data[0].category, null)
      html = await (await fetch(f.base + '/')).text()
      assert.ok(html.includes('真实模块6'))
      assert.ok(html.includes('数据库简介6'))
      assert.ok(html.includes(`href="/learn/modules/${published[6].module.slug}"`))
      assert.ok(!html.includes('不应公开的草稿'))
      assert.ok(!html.includes('不应出现在首页的私密正文'))
      assert.ok(!html.includes('secret-answer'))
      assert.equal((await f.request('/api/modules')).data.length, 7)
      browser = await chromium.launch({
        executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      const page = await context.newPage()
      const errors = []
      page.on('pageerror', error => errors.push(error.message))
      await page.goto(f.base)
      await page.locator('.featured-module').waitFor()
      await page.screenshot({ path: '/tmp/attackbox-landing-live.png', fullPage: true })
      await page.locator('.featured-module').click()
      await page.waitForURL(`**/learn/modules/${published[6].module.slug}`)
      await page.getByRole('heading', { level: 1, name: '真实模块6' }).waitFor()
      await page.goto(f.base)
      await page.getByRole('link', { name: '开始学习', exact: true }).click()
      await page.waitForURL('**/learn')
      for (const theme of ['midnight', 'graphite']) {
        await page.goto(f.base + '/auth')
        await page.evaluate(t => {
          localStorage.setItem('attackbox-appearance', t)
        }, theme)
        await page.reload()
        await page.locator('#email:enabled').waitFor()
        const colors = await page.locator('.auth-layout').evaluate(el => ({
          color: getComputedStyle(el).color,
          input: getComputedStyle(el.querySelector('#email')).backgroundColor,
          button: getComputedStyle(el.querySelector('button[type=submit]')).backgroundColor
        }))
        if (theme === 'midnight') {
          assert.equal(colors.color, 'rgb(242, 240, 237)')
          assert.equal(colors.input, 'rgb(21, 21, 27)')
          assert.equal(colors.button, 'rgb(242, 232, 223)')
        }
        await page.screenshot({ path: `/tmp/attackbox-auth-${theme}.png`, fullPage: true })
      }
      await page.setViewportSize({ width: 390, height: 844 })
      await page.goto(f.base)
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      await page.screenshot({ path: '/tmp/attackbox-landing-mobile.png', fullPage: true })
      await f.sql`update modules set status='unpublished' where id=${published[6].module.id}`
      html = await (await fetch(f.base + '/')).text()
      assert.ok(!html.includes('真实模块6'))
      assert.ok(html.includes('真实模块0'))
      await f.sql`update modules set status='unpublished'`
      html = await (await fetch(f.base + '/')).text()
      assert.ok(html.includes('新的内容，即将开始。'))
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      await f.cleanup()
    }
  }
)
