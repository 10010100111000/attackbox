import test from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, mkdirSync } from 'node:fs'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'

test('browser: publish, guest preview, login, persistent completion, resume and account isolation', { skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.CONTENT_TEST_BASE_URL, timeout: 120000 }, async () => {
  const f = await learningFixture()
  const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH || (existsSync('/usr/bin/chromium') ? '/usr/bin/chromium' : undefined), args: ['--no-sandbox'] })
  const errors = []
  let currentPage
  try {
    const user = await f.account(), other = await f.account(), mod = await f.createModule('draft')
    const modulePath = `/learn/modules/${mod.module.slug}`, first = modulePath + '/intro', second = modulePath + '/navigation'
    const admin = await browser.newContext()
    await admin.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
    const edit = await admin.newPage(); await edit.goto(f.base + '/admin/modules/' + mod.module.id)
    await edit.getByRole('button', { name: '保存并发布', exact: true }).click()
    await edit.locator('.badge.published').waitFor()
    const draft = await f.createModule('draft')
    await edit.goto(f.base + '/admin/modules')
    const publishedRow = edit.locator('tbody tr').filter({ hasText: mod.module.title })
    const draftRow = edit.locator('tbody tr').filter({ hasText: draft.module.title })
    await publishedRow.waitFor(); await draftRow.waitFor()
    for (const selector of ['.edit-btn', '.delete-btn']) {
      const publishedButton = await publishedRow.locator(selector).boundingBox()
      const draftButton = await draftRow.locator(selector).boundingBox()
      assert.ok(Math.abs(publishedButton.x - draftButton.x) < 1, 'actions align in draft and published rows')
    }
    const preview = await publishedRow.locator('.view-btn').boundingBox()
    const deleteButton = await publishedRow.locator('.delete-btn').boundingBox()
    assert.ok(Math.abs(preview.y + preview.height / 2 - deleteButton.y - deleteButton.height / 2) < 1, 'preview and buttons share a vertical center')
    await admin.close()
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } }), page = await context.newPage(); currentPage = page
    page.setDefaultTimeout(12000); page.on('pageerror', e => errors.push(e.message))
    async function login(account) {
      // Navigate through the same Nuxt app so auth/data caches are exercised across accounts.
      await page.locator('.card').filter({ hasText: mod.module.title }).first().click()
      await page.locator('.start-learning-btn').click()
      await page.getByLabel('打开用户菜单', { exact: true }).click()
      await page.getByRole('link', { name: '登录 / 注册', exact: true }).click()
      await page.locator('#email').fill(account.email); await page.locator('#password').fill(account.password)
      await page.getByRole('button', { name: '登录', exact: true }).click()
      await page.getByRole('link', { name: '进入学习平台', exact: true }).click()
      await page.waitForURL(f.base + first)
    }
    async function logout() {
      await page.getByLabel('打开用户菜单', { exact: true }).click()
      const loggedOut = page.waitForResponse(r => r.url().endsWith('/api/auth/logout') && r.status() === 200)
      await page.getByRole('button', { name: '退出', exact: true }).click()
      await loggedOut
      await page.locator('.avatar').filter({ hasText: '访' }).waitFor()
      await page.waitForURL(f.base + '/learn')
    }
    async function answer(index, value) {
      await page.locator('.answer-input').nth(index).fill(value)
      await page.locator('.answer-check-btn').nth(index).click()
      await page.locator('.answer-feedback.success').nth(index).waitFor()
    }
    await page.goto(f.base + first)
    await page.locator('.article-body > .markdown-body').waitFor()
    if (process.env.LEARNING_SCREENSHOT_DIR) {
      mkdirSync(process.env.LEARNING_SCREENSHOT_DIR, { recursive: true })
      await page.screenshot({ path: process.env.LEARNING_SCREENSHOT_DIR + '/reader-expanded.png' })
    }
    const bodyBefore = await page.locator('.article-body > .markdown-body').boundingBox()
    assert.equal(await page.locator('.article-body > .markdown-body').evaluate(el => getComputedStyle(el).fontSize), '17px')
    await page.getByRole('button', { name: '收起模块目录', exact: true }).click()
    const bodyAfter = await page.locator('.article-body > .markdown-body').boundingBox()
    if (process.env.LEARNING_SCREENSHOT_DIR) {
      await page.screenshot({ path: process.env.LEARNING_SCREENSHOT_DIR + '/reader-collapsed.png' })
      await page.evaluate(() => document.documentElement.dataset.theme = 'midnight')
      await page.screenshot({ path: process.env.LEARNING_SCREENSHOT_DIR + '/reader-dark.png' })
      await page.evaluate(() => document.documentElement.dataset.theme = 'graphite')
    }
    assert.ok(bodyAfter.width - bodyBefore.width > 180, 'collapsing the outline expands the article instead of shifting a fixed-width column')
    assert.ok(Math.abs(bodyBefore.x + bodyBefore.width - bodyAfter.x - bodyAfter.width) < 1, 'article right edge stays anchored')
    await page.getByRole('button', { name: '展开模块目录', exact: true }).click()
    await answer(0, 'whoami'); await answer(1, 'pwd')
    await page.locator('.guest-progress').waitFor()
    await page.locator('.guest-progress a').click()
    await page.locator('#email').fill(user.email); await page.locator('#password').fill(user.password)
    await page.getByRole('button', { name: '登录', exact: true }).click()
    await page.getByRole('link', { name: '进入学习平台', exact: true }).click()
    await page.waitForURL(f.base + first)
    assert.equal(await page.locator('.answer-feedback.success').count(), 0, 'guest completion cannot become saved account progress')
    // Deliver the older successful answer after the last answer has completed the chapter.
    await page.route('**/attempt', async route => {
      const response = await route.fetch()
      if (route.request().postDataJSON().questionId === mod.chapters[0].questions[0].id) await new Promise(resolve => setTimeout(resolve, 200))
      await route.fulfill({ response })
    })
    await page.locator('.answer-input').nth(0).fill(' WHOAMI ')
    await page.locator('.answer-input').nth(1).fill('pwd')
    await page.locator('.question-item').evaluateAll(forms => forms.forEach(form => form.requestSubmit()))
    await page.locator('.answer-feedback.success').nth(1).waitFor()
    await page.unrouteAll({ behavior: 'wait' })
    assert.equal(await page.locator('.quiz-badge').innerText(), '已完成', 'late responses cannot undo a saved completion')
    await page.getByText('已完成', { exact: true }).first().waitFor()
    assert.equal(await page.locator('.answer-input, .answer-check-btn').count(), 0, 'completed chapters have no answer controls')
    await page.reload(); await page.locator('.quiz-badge').filter({ hasText: '已完成' }).waitFor()
    assert.equal(await page.locator('.answer-feedback.success').count(), 2, 'refresh restores passed questions')
    assert.equal(await page.locator('.answer-input, .answer-check-btn').count(), 0, 'completed chapter stays read-only after reload')
    await page.locator('.next-btn').click(); await page.waitForURL(f.base + second)
    await page.getByRole('button', { name: '完成本章', exact: true }).click()
    await page.getByText('本章已完成', { exact: true }).waitFor()
    await page.getByRole('link', { name: '返回模块', exact: true }).click()
    await page.locator('.module-progress-status').filter({ hasText: '2 / 4' }).waitFor()
    await page.getByRole('link', { name: '全部模块', exact: true }).click()
    await page.locator('.recent-learning').waitFor()
    const cardText = await page.locator('.card').filter({ hasText: mod.module.title }).first().innerText()
    assert.ok(!cardText.includes('分钟') && !cardText.includes('含交互练习'), 'cards do not invent duration or practice claims')
    assert.ok((await page.locator('.recent-learning').getAttribute('href')).endsWith('/preview'))
    if (process.env.LEARNING_SCREENSHOT_DIR) {
      mkdirSync(process.env.LEARNING_SCREENSHOT_DIR, { recursive: true })
      await page.screenshot({ path: process.env.LEARNING_SCREENSHOT_DIR + '/learning-resume.png', fullPage: true })
    }
    await logout(); assert.equal(await page.locator('.recent-learning').count(), 0)
    await login(other)
    assert.equal(await page.locator('.answer-feedback.success').count(), 0)
    await page.goto(f.base + second); await page.getByRole('heading', { name: '先完成前面的章节', exact: true }).waitFor()
    await logout(); await login(user)
    await page.locator('.next-btn').click(); await page.waitForURL(f.base + second)
    await page.getByText('本章已完成', { exact: true }).waitFor()
    const device = await browser.newContext({ viewport: { width: 390, height: 844 } })
    // A fresh browser with a separate real login also restores the same account's progress.
    const loginResponse = await device.request.post(f.base + '/api/auth/login', { data: { email: user.email, password: user.password } })
    assert.equal(loginResponse.status(), 200)
    const mobile = await device.newPage(); mobile.on('pageerror', e => errors.push(e.message))
    await mobile.goto(f.base + first); await mobile.locator('.quiz-badge').filter({ hasText: '已完成' }).waitFor()
    assert.ok(await mobile.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await mobile.getByRole('button', { name: '打开模块目录', exact: true }).click()
    await mobile.getByRole('dialog').waitFor()
    assert.ok((await mobile.getByRole('dialog').innerText()).includes('已完成'))
    await device.close(); await context.close()
    assert.deepEqual(errors, [])
  } catch (error) {
    if (currentPage && !currentPage.isClosed()) console.error((await currentPage.locator('body').innerText()).slice(-3500))
    throw error
  } finally { await browser.close(); await f.cleanup() }
})
