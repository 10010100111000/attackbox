import test from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'
import { sandboxWorker } from './helpers/sandbox-worker.mjs'
const enabled = !!process.env.CONTENT_TEST_DATABASE_URL
const paragraph = '理解原理后，再亲手输入命令验证。'.repeat(12)
const body = `## 认识 **终端**\n\n${paragraph}\n\n\`\`\`bash\nwhoami\n## not-a-section\n\`\`\`\n\n${paragraph}\n\n## 实践步骤\n\n${paragraph}\n\n### 细节\n\n${paragraph}\n\n## 实践步骤\n\n${paragraph}\n\n${paragraph}`
const longBody = body + '\n\n' + Array.from({ length: 12 }, () => paragraph).join('\n\n')
const launches = () =>
  chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
    args: ['--no-sandbox']
  })
test(
  'reader section tree, hash navigation, local position isolation and tutorial copy behavior',
  { skip: !enabled, timeout: 90000 },
  async () => {
    const f = await learningFixture()
    let browser
    try {
      const a = await f.account(),
        b = await f.account()
      const mod = await f.createModule('publish', {
        chapters: [
          {
            title: '目录验收',
            slug: 'outline',
            access: 'public',
            bodyMarkdown: longBody,
            questions: [{ promptMarkdown: '## 实践步骤\n\n请手动输入命令', answer: 'whoami' }]
          },
          {
            title: '锁定章节',
            slug: 'locked',
            access: 'authenticated',
            bodyMarkdown: '## 不可泄露的小节',
            questions: []
          },
          {
            title: '简短章节',
            slug: 'short',
            access: 'public',
            bodyMarkdown: '没有二级标题。',
            questions: []
          }
        ]
      })
      browser = await launches()
      const ctx = await browser.newContext({
        viewport: { width: 1440, height: 1000 },
        permissions: ['clipboard-read', 'clipboard-write']
      })
      await ctx.addCookies([{ name: 'attackbox_session', value: a.token, url: f.base }])
      const page = await ctx.newPage(),
        errors = []
      page.setDefaultTimeout(10000)
      page.on('pageerror', error => errors.push(error.message))
      let contentRequests = 0
      page.on('request', request => {
        if (new URL(request.url()).pathname.startsWith('/api/modules/')) contentRequests++
      })
      const path = `/learn/modules/${mod.module.slug}/outline`
      await page.goto(f.base + path)
      const list = page.locator('.reader-sidebar .section-list'),
        scroll = page.getByLabel('章节正文', { exact: true })
      await list.getByRole('link', { name: '认识 终端', exact: true }).waitFor()
      assert.equal(await list.getByRole('link').count(), 4)
      assert.equal(await page.getByText('不可泄露的小节').count(), 0)
      assert.equal(await page.locator('.reader-article .copy-code-btn').count(), 1)
      await page.getByRole('button', { name: '收起本章小节' }).click()
      assert.equal(await list.isVisible(), false)
      await page.getByRole('button', { name: '展开本章小节' }).click()
      const before = contentRequests
      await list.locator('a[href="#' + encodeURIComponent('实践步骤-1') + '"]').click()
      await page.waitForFunction(() => decodeURIComponent(location.hash) === '#实践步骤-1')
      await page.waitForFunction(
        () =>
          document.querySelector('.reader-sidebar a[aria-current="location"]')?.textContent ===
          '实践步骤'
      )
      assert.equal(contentRequests, before, 'hash jumps do not refetch chapter data')
      await page.screenshot({ path: '/tmp/attackbox-reader-sections.png', animations: 'disabled' })
      assert.equal(await page.evaluate(() => window.scrollY), 0)
      const top = await page.locator('h2[id="实践步骤-1"]').boundingBox(),
        container = await scroll.boundingBox()
      assert.ok(Math.abs(top.y - container.y - 24) <= 2)
      await page.goto(f.base + path)
      await scroll.evaluate(el => {
        el.scrollTop = 620
      })
      const positionKey = `attackbox-reader-position:${a.id}:${mod.chapters[0].chapterId}`
      await page.waitForFunction(
        key => JSON.parse(localStorage.getItem(key) || 'null')?.top > 600,
        positionKey
      )
      const saved = await scroll.evaluate(el => el.scrollTop)
      await page
        .locator('.reader-sidebar')
        .getByRole('link', { name: /简短章节/ })
        .click()
      await page.waitForURL('**/short')
      await page.getByRole('heading', { name: '简短章节', exact: true }).waitFor()
      assert.equal(await page.getByRole('button', { name: '收起本章小节' }).count(), 0)
      await page
        .locator('.reader-sidebar')
        .getByRole('link', { name: /目录验收/ })
        .click()
      await page.waitForURL('**/outline')
      await page.getByRole('heading', { name: '目录验收', exact: true }).waitFor()
      await page.waitForFunction(
        value => Math.abs(document.querySelector('.reader-scroll').scrollTop - value) < 3,
        saved
      )
      await page.reload()
      await page.waitForFunction(
        value => Math.abs(document.querySelector('.reader-scroll').scrollTop - value) < 3,
        saved
      )
      await page.evaluate(() => navigator.clipboard.writeText('unchanged'))
      await scroll.focus()
      await page.locator('.reader-article pre code').evaluate(el => {
        const range = document.createRange()
        range.selectNodeContents(el)
        const selection = window.getSelection()
        selection.removeAllRanges()
        selection.addRange(range)
      })
      await page.keyboard.press('Control+c')
      assert.match(await page.evaluate(() => navigator.clipboard.readText()), /whoami/)
      const input = page.locator('.answer-input')
      await input.fill('typed-answer')
      await input.evaluate(el => el.select())
      await page.keyboard.press('Control+c')
      assert.equal(await page.evaluate(() => navigator.clipboard.readText()), 'typed-answer')
      await ctx.addCookies([{ name: 'attackbox_session', value: b.token, url: f.base }])
      await page.reload()
      await page.locator('.chapter-main-title').waitFor()
      assert.ok(
        await scroll.evaluate(el => el.scrollTop < 3),
        'another account does not inherit reading position'
      )
      await page.goto(f.base + path + '#' + encodeURIComponent('实践步骤'))
      await page.waitForFunction(
        () =>
          document
            .querySelector('.reader-sidebar a[aria-current="location"]')
            ?.getAttribute('href') ===
          '#' + encodeURIComponent('实践步骤')
      )
      assert.equal(await page.evaluate(() => window.scrollY), 0)
      await page.setViewportSize({ width: 390, height: 844 })
      await page.getByRole('button', { name: '打开模块目录' }).click()
      const mobile = page.getByRole('dialog', { name: '模块目录' })
      await mobile.getByRole('link', { name: '认识 终端', exact: true }).click()
      await mobile.waitFor({ state: 'hidden' })
      await page.waitForFunction(() => decodeURIComponent(location.hash) === '#认识-终端')
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      await f.cleanup()
    }
  }
)

test(
  'CLI clipboard and explicit GUI clipboard remain bidirectional within the reader',
  { skip: !enabled || !process.env.NODE_TEST_CLUSTER_SECRET, timeout: 90000 },
  async () => {
    const f = await learningFixture()
    let browser, worker
    try {
      worker = await sandboxWorker(f.base, process.env.NODE_TEST_CLUSTER_SECRET)
      const learner = await f.account(),
        mod = await f.createModule()
      await f.request('/api/sandbox/start', { actor: learner, method: 'POST' })
      browser = await launches()
      const ctx = await browser.newContext({
        viewport: { width: 1600, height: 1000 },
        permissions: ['clipboard-read', 'clipboard-write']
      })
      await ctx.addCookies([{ name: 'attackbox_session', value: learner.token, url: f.base }])
      const page = await ctx.newPage()
      page.setDefaultTimeout(10000)
      await page.goto(`${f.base}/learn/modules/${mod.module.slug}/intro`)
      await page.getByRole('button', { name: 'CLI 终端', exact: true }).click()
      await page.locator('.terminal-status').filter({ hasText: '已连接' }).waitFor()
      await page.evaluate(() => navigator.clipboard.writeText('echo cli-paste\n'))
      await page.locator('.terminal-host .xterm-helper-textarea').focus()
      await page.keyboard.press('Control+Shift+v')
      for (let i = 0; i < 100 && !worker.commands.includes('echo cli-paste'); i++)
        await page.waitForTimeout(30)
      assert.ok(worker.commands.includes('echo cli-paste'))
      await page.waitForFunction(() =>
        document.querySelector('.terminal-host')?.textContent.includes('Executed: echo cli-paste')
      )
      const screen = await page.locator('.terminal-host .xterm-screen').boundingBox()
      await page.mouse.move(screen.x + 1, screen.y + 8)
      await page.mouse.down()
      await page.mouse.move(screen.x + 110, screen.y + 8, { steps: 5 })
      await page.mouse.up()
      await page.keyboard.press('Control+Shift+c')
      assert.match(await page.evaluate(() => navigator.clipboard.readText()), /box@fixture/)
      await page.getByRole('button', { name: 'GUI 桌面', exact: true }).click()
      await page.getByText('桌面已连接', { exact: true }).waitFor()
      await page.getByRole('button', { name: '桌面剪贴板', exact: true }).click()
      const text = page.getByRole('textbox', { name: '剪贴板文本', exact: true })
      await text.fill('browser-to-desktop')
      await page.getByRole('button', { name: '发送到桌面', exact: true }).click()
      await page.waitForFunction(
        () =>
          document.querySelector('.desktop-clipboard textarea')?.value ===
          'desktop:browser-to-desktop'
      )
      assert.ok(
        worker.desktopInputs.some(
          input => input.type === 'clipboard' && input.text === 'browser-to-desktop'
        )
      )
      await page.getByRole('button', { name: '复制到浏览器', exact: true }).click()
      assert.equal(
        await page.evaluate(() => navigator.clipboard.readText()),
        'desktop:browser-to-desktop'
      )
      await page.screenshot({
        path: '/tmp/attackbox-desktop-clipboard.png',
        animations: 'disabled'
      })
      await page.evaluate(() => navigator.clipboard.writeText('another-transfer'))
      await page.getByRole('button', { name: '读取浏览器', exact: true }).click()
      await page.waitForFunction(
        () => document.querySelector('.desktop-clipboard textarea')?.value === 'another-transfer'
      )
    } finally {
      await browser?.close()
      await worker?.close()
      await f.cleanup()
    }
  }
)
