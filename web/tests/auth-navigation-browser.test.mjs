import assert from 'node:assert/strict'
import test from 'node:test'
import { chromium } from 'playwright'

const base = process.env.AUTH_TEST_BASE_URL
if (base) assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(new URL(base).hostname), 'use a local test server')
const user = { id: 'auth-navigation-user', email: 'learner@example.test', displayName: 'Navigation learner', role: 'user', status: 'active' }
const module = { id: 'navigation-module', slug: 'navigation-module', title: 'Navigation test module', summary: 'Loaded independently', difficulty: 'beginner', categoryId: null, category: null, coverAssetId: null, publishedAt: new Date().toISOString(), chapterCount: 1 }
function deferred() {
  let resolve
  const promise = new Promise(r => { resolve = r })
  return { promise, resolve }
}
async function fixture(run) {
  const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium', args: ['--no-sandbox'] })
  const context = await browser.newContext()
  const page = await context.newPage()
  page.setDefaultTimeout(5000)
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  try { await run({ page, context }); assert.deepEqual(errors, []) } finally { await browser.close() }
}
const reply = (route, json, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(json) })
async function enterCredentials(page) {
  await page.goto(`${base}/auth`)
  await page.locator('#email:enabled').waitFor({ timeout: 3000 })
  await page.locator('#email').fill(user.email)
  await page.locator('#password').fill('test-password-only')
}

test('login navigates before learning APIs finish; slow routes do not block modules or categories', { skip: !base }, async () => {
  await fixture(async ({ page }) => {
    const routes = deferred(), modules = deferred(), requested = new Set()
    let routesAvailable = false
    await page.route('**/api/**', async route => {
      const path = new URL(route.request().url()).pathname
      requested.add(path)
      if (path === '/api/auth/me') return reply(route, {}, 401)
      if (path === '/api/auth/login') return reply(route, { user })
      if (path === '/api/routes') {
        await routes.promise
        return routesAvailable ? reply(route, { data: [] }) : reply(route, { statusMessage: 'temporarily unavailable' }, 503)
      }
      if (path === '/api/modules') { await modules.promise; return reply(route, { data: [module] }) }
      return reply(route, { data: [] })
    })
    try {
      await enterCredentials(page)
      await page.getByRole('button', { name: '登录', exact: true }).click()
      await page.waitForURL('**/learn', { timeout: 3000 })
      await page.getByRole('navigation', { name: '主导航' }).waitFor()
      await page.getByText('正在加载模块列表…', { exact: true }).waitFor()
      await page.waitForFunction(() => !!document.querySelector('.filters'))
      assert.ok(requested.has('/api/categories'), 'independent requests start while routes are pending')
      modules.resolve()
      await page.getByRole('heading', { name: module.title, exact: true }).first().waitFor()
      assert.ok(requested.has('/api/me/learning'), 'login sets learner identity before loading history')
      await page.getByRole('button', { name: '学习路线', exact: true }).click()
      await page.getByText('正在加载学习路线…', { exact: true }).waitFor()
      routes.resolve()
      await page.getByText('学习路线加载失败，请稍后重试。', { exact: true }).waitFor()
      routesAvailable = true
      await page.getByRole('button', { name: '重新加载', exact: true }).click()
      await page.getByText('暂时没有已发布的路线。', { exact: true }).waitFor()
      await page.locator('#user-menu summary').click()
      await page.getByText(user.displayName, { exact: true }).waitFor()
    } finally { routes.resolve(); modules.resolve() }
  })
})

test('slow initial session lookup does not disable login or overwrite a successful login', { skip: !base }, async () => {
  await fixture(async ({ page }) => {
    const session = deferred(), sessionStarted = deferred(), sessionDone = deferred()
    await page.route('**/api/**', async route => {
      const path = new URL(route.request().url()).pathname
      if (path === '/api/auth/me') {
        sessionStarted.resolve()
        await session.promise
        await reply(route, {}, 401)
        sessionDone.resolve()
        return
      }
      if (path === '/api/auth/login') return reply(route, { user })
      return reply(route, { data: [] })
    })
    try {
      await enterCredentials(page)
      await sessionStarted.promise
      await page.getByRole('button', { name: '登录', exact: true }).click()
      await page.waitForURL('**/learn', { timeout: 3000 })
      session.resolve()
      await sessionDone.promise
      // Wait for the delayed /me handler to settle, then check the rendered account state.
      await page.waitForTimeout(150)
      await page.locator('#user-menu summary').click()
      await page.getByText(user.displayName, { exact: true }).waitFor({ timeout: 2000 })
    } finally { session.resolve() }
  })
})

test('invalid credentials show an error, allow retry, and never navigate or submit twice while pending', { skip: !base }, async () => {
  await fixture(async ({ page }) => {
    const login = deferred()
    let attempts = 0
    await page.route('**/api/**', async route => {
      const path = new URL(route.request().url()).pathname
      if (path === '/api/auth/me') return reply(route, {}, 401)
      if (path === '/api/auth/login') {
        attempts++
        if (attempts === 1) { await login.promise; return reply(route, { statusMessage: '邮箱或密码错误' }, 401) }
        return reply(route, { user })
      }
      return reply(route, { data: [] })
    })
    try {
      await enterCredentials(page)
      await page.getByRole('button', { name: '登录', exact: true }).click()
      const busy = page.getByRole('button', { name: '正在登录…', exact: true })
      await busy.waitFor()
      assert.ok(await busy.isDisabled())
      await page.locator('#auth-form').evaluate(form => { form.requestSubmit(); form.requestSubmit() })
      login.resolve()
      await page.getByRole('status').filter({ hasText: '邮箱或密码错误' }).waitFor()
      assert.equal(attempts, 1)
      assert.equal(new URL(page.url()).pathname, '/auth')
      await page.getByRole('button', { name: '登录', exact: true }).click()
      await page.waitForURL('**/learn', { timeout: 3000 })
      assert.equal(attempts, 2)
      assert.equal(new URL(page.url()).search, '')
    } finally { login.resolve() }
  })
})
