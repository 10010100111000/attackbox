import test from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'
import { sandboxWorker } from './helpers/sandbox-worker.mjs'

const near = (actual, expected) =>
  assert.ok(Math.abs(actual - expected) <= 2, `${actual} should be near ${expected}`)
test(
  'practice split layout resizes both axes, remembers preference and preserves CLI/GUI connections',
  {
    skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.NODE_TEST_CLUSTER_SECRET,
    timeout: 90000
  },
  async () => {
    const f = await learningFixture()
    let worker, browser
    try {
      worker = await sandboxWorker(f.base, process.env.NODE_TEST_CLUSTER_SECRET)
      const learner = await f.account()
      const mod = await f.createModule('publish', {
        chapters: [
          {
            title: '并排阅读与实践',
            slug: 'split',
            access: 'public',
            bodyMarkdown: Array.from(
              { length: 100 },
              (_, i) =>
                `## 步骤 ${i + 1}\n\n阅读教程，同时观察终端里的命令输出。\n\n\`\`\`bash\nnmap -sV --script default 10.201.0.2\n\`\`\``
            ).join('\n\n'),
            questions: []
          }
        ]
      })
      await f.request('/api/sandbox/start', { actor: learner, method: 'POST' })
      browser = await chromium.launch({
        executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const context = await browser.newContext({ viewport: { width: 1920, height: 1080 } })
      await context.addCookies([{ name: 'attackbox_session', value: learner.token, url: f.base }])
      const page = await context.newPage(),
        errors = []
      page.setDefaultTimeout(10000)
      page.on('pageerror', e => errors.push(e.message))
      await page.goto(`${f.base}/learn/modules/${mod.module.slug}/split`)
      const cli = page.getByRole('button', { name: 'CLI 终端', exact: true })
      await cli.click()
      await page.locator('.terminal-status').filter({ hasText: '已连接' }).waitFor()
      const panel = page.getByRole('region', { name: '练习环境面板' }),
        reader = page.getByLabel('章节正文', { exact: true })
      await page.waitForFunction(() =>
        document.querySelector('.practice-dock')?.classList.contains('dock-right')
      )
      let box = await panel.boundingBox(),
        reading = await reader.boundingBox()
      assert.ok(box.height > 850)
      assert.ok(box.width > 700, 'side panel uses wide screens without the old 680px ceiling')
      near(box.y, reading.y)
      near(box.height, reading.height)
      near(reading.x + reading.width, box.x)
      const terminal = page.locator('.terminal-host .xterm-helper-textarea')
      await terminal.focus()
      await page.keyboard.type('echo dock-layout-state')
      await page.keyboard.press('Enter')
      await page.waitForFunction(() =>
        document.querySelector('.terminal-host')?.textContent.includes('dock-layout-state')
      )
      const grip = page.getByRole('separator', { name: '调整练习面板宽度' })
      const bounds = await grip.boundingBox(),
        before = box.width
      await page.mouse.move(bounds.x + bounds.width / 2, bounds.y + 100)
      await page.mouse.down()
      await page.mouse.move(bounds.x - 120, bounds.y + 100, { steps: 8 })
      await page.mouse.up()
      box = await panel.boundingBox()
      assert.ok(box.width > before + 100)
      await grip.focus()
      await page.keyboard.press('ArrowLeft')
      near((await panel.boundingBox()).width, box.width + 20)
      await page.keyboard.press('Home')
      near((await panel.boundingBox()).width, 420)
      await page.keyboard.press('End')
      near((await reader.boundingBox()).width, 480)
      assert.equal(worker.terminals.length, 1)
      await reader.evaluate(el => {
        el.scrollTop = 700
      })
      const scrollTop = await reader.evaluate(el => el.scrollTop)
      await page.getByRole('button', { name: '切换为底部停靠' }).click()
      box = await panel.boundingBox()
      reading = await reader.boundingBox()
      near(box.x, reading.x)
      near(box.width, reading.width)
      near(reading.y + reading.height, box.y)
      near(await reader.evaluate(el => el.scrollTop), scrollTop)
      const bottomGrip = page.getByRole('separator', { name: '调整练习面板高度' })
      await bottomGrip.focus()
      await page.keyboard.press('ArrowUp')
      near((await panel.boundingBox()).height, box.height + 20)
      await page.keyboard.press('Home')
      near((await panel.boundingBox()).height, 180)
      await page.keyboard.press('End')
      assert.ok((await reader.boundingBox()).height >= 179)
      const savedHeight = (await panel.boundingBox()).height
      await page.reload()
      await cli.click()
      await page.locator('.terminal-status').filter({ hasText: '已连接' }).waitFor()
      assert.equal(await panel.evaluate(el => el.classList.contains('dock-right')), false)
      near((await panel.boundingBox()).height, savedHeight)
      await page.getByRole('button', { name: '切换为右侧分屏' }).click()
      await page.getByRole('button', { name: '收起模块目录' }).click()
      assert.ok((await reader.boundingBox()).width >= 479)
      await page.screenshot({
        path: '/tmp/attackbox-practice-side-cli.png',
        animations: 'disabled'
      })
      await page.getByRole('button', { name: 'GUI 桌面', exact: true }).click()
      await page.getByText('桌面已连接', { exact: true }).waitFor()
      assert.equal(worker.desktops.length, 1)
      const canvas = page.locator('.desktop-host canvas')
      const canvasBox = await canvas.boundingBox(),
        hostBox = await page.locator('.desktop-host').boundingBox()
      assert.ok(canvasBox.width <= hostBox.width + 2 && canvasBox.height <= hostBox.height + 2)
      await page.screenshot({
        path: '/tmp/attackbox-practice-side-gui.png',
        animations: 'disabled'
      })
      const terminals = worker.terminals.length
      await page.getByRole('button', { name: '切换为底部停靠' }).click()
      await page.getByRole('button', { name: '切换为右侧分屏' }).click()
      await page.getByRole('button', { name: '收起练习面板' }).click()
      await page.getByRole('button', { name: '恢复练习面板' }).click()
      assert.equal(worker.desktops.length, 1)
      assert.equal(worker.terminals.length, terminals)
      await page.setViewportSize({ width: 960, height: 900 })
      await page.waitForFunction(
        () => !document.querySelector('.practice-dock').classList.contains('dock-right')
      )
      assert.equal(await page.locator('.placement-toggle').count(), 0)
      assert.equal(await page.getByRole('separator').getAttribute('aria-orientation'), 'horizontal')
      await page.setViewportSize({ width: 1920, height: 1080 })
      await page.waitForFunction(() =>
        document.querySelector('.practice-dock').classList.contains('dock-right')
      )
      assert.equal(worker.desktops.length, 1)
      await page.reload()
      await cli.click()
      await page.waitForFunction(() =>
        document.querySelector('.practice-dock').classList.contains('dock-right')
      )
      await page.setViewportSize({ width: 390, height: 844 })
      await page.waitForFunction(
        () => !document.querySelector('.practice-dock').classList.contains('dock-right')
      )
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      assert.ok((await reader.boundingBox()).height >= 179)
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      await worker?.close()
      await f.cleanup()
    }
  }
)

test(
  'challenge arena shares the same split layout and resize controls',
  {
    skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.NODE_TEST_CLUSTER_SECRET,
    timeout: 60000
  },
  async () => {
    const f = await learningFixture()
    let worker, browser, challengeId
    try {
      worker = await sandboxWorker(f.base, process.env.NODE_TEST_CLUSTER_SECRET)
      const learner = await f.account()
      const result = await f.request('/api/admin/challenges', {
        actor: f.admin,
        method: 'POST',
        body: {
          title: '挑战分屏验收',
          summary: '',
          category: '',
          difficulty: 'beginner',
          bodyMarkdown: '# 实验说明\n\n边看题目边操作终端。',
          targetTemplateId: null,
          intent: 'publish',
          stages: [
            { title: '找到入口', promptMarkdown: '提交入口 Flag', hint: '', flag: 'flag{test}' }
          ]
        }
      })
      challengeId = result.data.id
      await f.request('/api/sandbox/start', { actor: learner, method: 'POST' })
      browser = await chromium.launch({
        executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const context = await browser.newContext({ viewport: { width: 1600, height: 1000 } })
      await context.addCookies([{ name: 'attackbox_session', value: learner.token, url: f.base }])
      const page = await context.newPage(),
        errors = []
      page.on('pageerror', error => errors.push(error.message))
      await page.goto(`${f.base}/challenges/${challengeId}`)
      await page.getByRole('button', { name: 'CLI 终端', exact: true }).click()
      await page.locator('.terminal-status').filter({ hasText: '已连接' }).waitFor()
      const grip = page.getByRole('separator', { name: '调整练习面板宽度' })
      await grip.focus()
      await page.keyboard.press('Home')
      near((await page.locator('.practice-dock').boundingBox()).width, 420)
      assert.ok((await page.locator('.challenge-scroll').boundingBox()).width >= 480)
      await page.getByRole('button', { name: '切换为底部停靠' }).click()
      await page.getByRole('separator', { name: '调整练习面板高度' }).waitFor()
      assert.equal(worker.terminals.length, 1)
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      await worker?.close()
      if (challengeId) await f.sql`delete from challenges where id=${challengeId}`
      await f.cleanup()
    }
  }
)
