import test from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { writeFile } from 'node:fs/promises'
import { learningFixture } from './helpers/learning-fixture.mjs'
import { sandboxWorker } from './helpers/sandbox-worker.mjs'

test(
  'chapter practice combines sandbox and target, exposes IPs, protects partial failure and hides on reading-only chapters',
  {
    skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.NODE_TEST_CLUSTER_SECRET,
    timeout: 90000
  },
  async () => {
    const f = await learningFixture()
    let worker, browser, page, learner
    try {
      worker = await sandboxWorker(f.base, process.env.NODE_TEST_CLUSTER_SECRET, {
        composeLabs: true
      })
      learner = await f.account()
      const template = await f.seedTarget(
        { name: 'exercise-lab', description: '不应重复显示的模板描述' },
        worker.boot
      )
      const mod = await f.createModule('publish', {
        chapters: [
          {
            title: '目录调查',
            slug: 'lab',
            access: 'public',
            bodyMarkdown: '## 调查范围\n\n阅读材料后开始练习。',
            questions: [{ promptMarkdown: '输入 ready', answer: 'ready' }]
          },
          {
            title: '只读章节',
            slug: 'reading',
            access: 'public',
            targetMode: 'none',
            bodyMarkdown: '## 阅读\n\n本章无需实验。',
            questions: []
          },
          {
            title: '本地练习',
            slug: 'local',
            access: 'public',
            targetMode: 'none',
            bodyMarkdown: '## 本地目录\n\n在沙箱内练习。',
            questions: [{ promptMarkdown: '输入 yes', answer: 'yes' }]
          }
        ]
      })
      await f.sql`update modules set default_target_template_id=${template.id} where id=${mod.module.id}`
      browser = await chromium.launch({
        executablePath: '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const context = await browser.newContext({
        viewport: { width: 1440, height: 1000 },
        permissions: ['clipboard-read', 'clipboard-write']
      })
      await context.addCookies([{ name: 'attackbox_session', value: learner.token, url: f.base }])
      page = await context.newPage()
      page.setDefaultTimeout(10000)
      const errors = [],
        requests = []
      page.on('pageerror', e => errors.push(e.message))
      page.on('request', r => {
        if (r.url().includes('/api/sandbox/') || r.url().includes('/api/targets'))
          requests.push(r.url())
      })
      const path = `/learn/modules/${mod.module.slug}`
      await page.goto(f.base + path + '/reading')
      await page.getByRole('heading', { name: '只读章节', exact: true }).waitFor()
      assert.equal(await page.getByRole('region', { name: '实验环境', exact: true }).count(), 0)
      assert.equal(await page.locator('.reader-toolbar').isVisible(), false)
      assert.equal(
        requests.length,
        0,
        'reading-only chapter must not subscribe to sandbox/target status'
      )
      await page.goto(f.base + path + '/lab')
      const panel = page.getByRole('region', { name: '实验环境', exact: true })
      await panel.getByRole('button', { name: '启动靶机', exact: true }).click()
      await panel.getByText('10.202.0.2', { exact: true }).waitFor()
      assert.equal(worker.boxes.size, 0, 'starting target does not auto-start sandbox')
      await panel.getByRole('button', { name: '启动环境', exact: true }).click()
      await page.locator('.terminal-status').filter({ hasText: '已连接' }).waitFor()
      await panel.getByText('10.202.0.14', { exact: true }).waitFor()
      assert.equal(await panel.getByText('exercise-lab', { exact: true }).count(), 0)
      assert.equal(await panel.locator('details').count(), 0)
      assert.equal(await page.getByRole('link', { name: '返回模块', exact: true }).count(), 0)
      await panel.getByRole('button', { name: '复制靶机 IP', exact: true }).click()
      assert.equal(await page.evaluate(() => navigator.clipboard.readText()), '10.202.0.2')
      await panel.getByRole('button', { name: '停止靶机' }).click()
      const targetDialog = page.getByRole('alertdialog', { name: '停止靶机？' })
      await targetDialog.getByRole('button', { name: '取消', exact: true }).click()
      assert.equal(worker.requests.filter(r => r.type === 'compose_stop').length, 0)
      await panel.getByRole('button', { name: '停止靶机' }).click()
      await targetDialog.getByRole('button', { name: '停止靶机', exact: true }).click()
      await panel.getByRole('button', { name: '启动靶机', exact: true }).waitFor()
      assert.equal(worker.boxes.size, 1, 'stopping target keeps sandbox running independently')
      await panel.getByRole('button', { name: '停止环境' }).click()
      const sandboxDialog = page.getByRole('alertdialog', { name: '释放环境？' })
      await sandboxDialog.getByRole('button', { name: '释放环境', exact: true }).click()
      await panel.locator('button:enabled').filter({ hasText: '启动环境' }).waitFor()
      assert.equal(worker.boxes.size, 0)
      const stops = worker.requests.map(r => r.type)
      assert.ok(
        stops.indexOf('compose_stop') < stops.indexOf('destroy_box'),
        'target stops before sandbox release'
      )
      await page.goto(f.base + path + '/local')
      await panel.getByRole('button', { name: '启动环境', exact: true }).click()
      await page.locator('.terminal-status').filter({ hasText: '已连接' }).waitFor()
      assert.equal(
        worker.requests.filter(r => r.type === 'compose_start').length,
        1,
        'local practice needs no target container'
      )
      await page.screenshot({ path: '/tmp/attackbox-unified-practice.png', animations: 'disabled' })
      await page.goto(f.base + path + '/reading')
      assert.equal(await panel.count(), 0)
      assert.equal(await page.locator('.practice-dock').count(), 0)
      assert.deepEqual(errors, [])
    } catch (error) {
      if (page) {
        await page.screenshot({ path: '/tmp/attackbox-practice-failure.png', fullPage: true })
        await writeFile(
          '/tmp/attackbox-practice-failure.txt',
          JSON.stringify(
            {
              body: await page.locator('body').innerText(),
              requests: worker?.requests.map(r => ({ type: r.type, owner: r.payload?.owner_id }))
            },
            null,
            2
          )
        )
      }
      throw error
    } finally {
      await browser?.close()
      await worker?.close()
      if (learner) await f.sql`delete from target_instances where user_id=${learner.id}`
      await f.cleanup()
    }
  }
)
