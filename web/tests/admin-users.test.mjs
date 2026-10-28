import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'
import { sandboxWorker } from './helpers/sandbox-worker.mjs'
const enabled = Boolean(process.env.CONTENT_TEST_DATABASE_URL && process.env.CONTENT_TEST_BASE_URL)
test('user administration: real accounts, progress, lifecycle, permissions and browser', { skip: !enabled, timeout: 120000 }, async () => {
  const f = await learningFixture()
  let browser, worker
  try {
    const alice = await f.account(), bob = await f.account(), pending = await f.account()
    await f.sql`UPDATE users SET status='pending',email_verified_at=null WHERE id=${pending.id}`
    await f.request('/api/admin/users', { status: 401 })
    await f.request('/api/admin/users', { actor: alice, status: 403 })
    await f.request('/api/admin/users?page=-1', { actor: f.admin, status: 400 })
    await f.request('/api/admin/users?status=invalid', { actor: f.admin, status: 400 })
    await f.request('/api/admin/users/not-an-id', { actor: f.admin, status: 400 })
    await f.request(`/api/admin/users/${crypto.randomUUID()}`, { actor: f.admin, status: 404 })
    const listed = (await f.request('/api/admin/users?search=' + encodeURIComponent(alice.email), { actor: f.admin })).data
    assert.equal(listed.total, 1); assert.equal(listed.items[0].id, alice.id)
    const serialized = JSON.stringify(listed)
    for (const secret of ['passwordHash', 'tokenHash', alice.password, alice.token]) assert.ok(!serialized.includes(secret))
    const noWildcard = (await f.request('/api/admin/users?search=%25', { actor: f.admin })).data
    assert.equal(noWildcard.total, 0, 'search treats SQL wildcard characters literally')
    const pendingList = (await f.request('/api/admin/users?status=pending&search=' + encodeURIComponent(pending.email), { actor: f.admin })).data
    assert.equal(pendingList.total, 1); assert.equal(pendingList.items[0].status, 'pending')
    const adminList = (await f.request('/api/admin/users?role=admin&search=' + encodeURIComponent(f.admin.email), { actor: f.admin })).data
    assert.equal(adminList.total, 1); assert.equal(adminList.items[0].id, f.admin.id)
    const pagingName = 'Paging-' + crypto.randomUUID()
    for (let index = 0; index < 21; index++) {
      const account = await f.account()
      await f.sql`UPDATE users SET display_name=${pagingName} WHERE id=${account.id}`
    }
    const firstPage = (await f.request('/api/admin/users?search=' + pagingName, { actor: f.admin })).data
    const secondPage = (await f.request('/api/admin/users?page=2&search=' + pagingName, { actor: f.admin })).data
    assert.equal(firstPage.total, 21); assert.equal(firstPage.items.length, 20)
    assert.equal(secondPage.page, 2); assert.equal(secondPage.items.length, 1)
    assert.ok(!firstPage.items.some(item => item.id === secondPage.items[0].id))
    const csrf = await fetch(f.base + `/api/admin/users/${alice.id}/status`, { method: 'PUT', headers: { cookie: `attackbox_session=${f.admin.token}`, origin: 'https://elsewhere.example', 'content-type': 'application/json' }, body: JSON.stringify({ suspended: true }) })
    assert.equal(csrf.status, 403)
    const module = await f.createModule()
    await f.sql`INSERT INTO chapter_progress(user_id,chapter_id,completed_at) VALUES (${alice.id},${module.chapters[0].chapterId},now())`
    let detail = (await f.request(`/api/admin/users/${alice.id}`, { actor: f.admin })).data
    assert.deepEqual(detail.progress, { modules: 1, visited: 1, completed: 1 })
    assert.equal(detail.history.length, 1); assert.equal(detail.activeSessions, 1)
    assert.equal(detail.history[0].progress.completedChapters, 1)
    assert.ok(!JSON.stringify(detail).includes('passwordHash'))
    await f.request(`/api/admin/users/${f.admin.id}/status`, { actor: f.admin, method: 'PUT', body: { suspended: true }, status: 409 })
    await f.request(`/api/admin/users/${alice.id}/status`, { actor: bob, method: 'PUT', body: { suspended: true }, status: 403 })
    await f.request(`/api/admin/users/${alice.id}/status`, { actor: f.admin, method: 'PUT', body: { suspended: 'true' }, status: 400 })
    // The Worker fixture exercises the real control API and RPC; kernel sandboxing is not emulated here.
    let failDestroy = false
    worker = await sandboxWorker(f.base, process.env.SANDBOX_CLUSTER_SECRET, { failDestroy: () => failDestroy })
    const instance = (await f.request('/api/sandbox/start', { actor: alice, method: 'POST' })).data
    assert.ok(instance.id)
    const stopped = (await f.request(`/api/admin/users/${alice.id}/status`, { actor: f.admin, method: 'PUT', body: { suspended: true } })).data
    assert.equal(stopped.status, 'suspended'); assert.equal(stopped.cleanup, 'released')
    await f.request('/api/auth/me', { actor: alice, status: 401 })
    await f.request('/api/sandbox/start', { actor: alice, method: 'POST', status: 401 })
    await f.request('/api/auth/login', { method: 'POST', body: { email: alice.email, password: alice.password }, status: 403 })
    detail = (await f.request(`/api/admin/users/${alice.id}`, { actor: f.admin })).data
    assert.equal(detail.activeSessions, 0); assert.equal(detail.instance, null)
    assert.equal(detail.progress.completed, 1)
    await f.request(`/api/admin/users/${alice.id}/status`, { actor: f.admin, method: 'PUT', body: { suspended: false } })
    await f.request('/api/auth/me', { actor: alice, status: 401 }) // restoring never revives revoked sessions
    const login = await f.request('/api/auth/login', { method: 'POST', body: { email: alice.email, password: alice.password } })
    const freshToken = /attackbox_session=([^;]+)/.exec(login.response.headers.get('set-cookie'))[1]
    await f.request(`/api/admin/users/${alice.id}/revoke-sessions`, { actor: f.admin, method: 'POST' })
    await f.request('/api/auth/me', { actor: { token: freshToken }, status: 401 })
    // A failed Worker cleanup must not roll back suspension or hide allocated resources.
    const bobInstance = (await f.request('/api/sandbox/start', { actor: bob, method: 'POST' })).data
    failDestroy = true
    const failedCleanup = (await f.request(`/api/admin/users/${bob.id}/status`, { actor: f.admin, method: 'PUT', body: { suspended: true } })).data
    assert.equal(failedCleanup.cleanup, 'pending')
    const waiting = (await f.request(`/api/admin/users/${bob.id}`, { actor: f.admin })).data
    assert.equal(waiting.user.status, 'suspended'); assert.equal(waiting.instance.state, 'cleanup_failed')
    await f.request('/api/auth/me', { actor: bob, status: 401 })
    failDestroy = false
    await f.request(`/api/admin/instances/${bobInstance.id}/stop`, { actor: f.admin, method: 'POST' })
    await f.request(`/api/admin/users/${bob.id}/status`, { actor: f.admin, method: 'PUT', body: { suspended: false } })
    const verifyToken = crypto.randomBytes(32).toString('hex')
    await f.sql`INSERT INTO email_tokens(user_id,target_email,purpose,token_hash,expires_at) VALUES (${pending.id},${pending.email},'verify_email',${crypto.createHash('sha256').update(verifyToken).digest()},now()+interval '1 hour')`
    await f.request(`/api/admin/users/${pending.id}/status`, { actor: f.admin, method: 'PUT', body: { suspended: true } })
    await f.request('/api/auth/verify-email', { method: 'POST', body: { token: verifyToken }, status: 403 })
    const restored = (await f.request(`/api/admin/users/${pending.id}/status`, { actor: f.admin, method: 'PUT', body: { suspended: false } })).data
    assert.equal(restored.status, 'pending')
    await f.request('/api/auth/verify-email', { method: 'POST', body: { token: verifyToken } })
    await f.request('/api/auth/verify-email', { method: 'POST', body: { token: verifyToken }, status: 400 })
    // Hold the account row until password verification has reached session issuance.
    // Suspending during this window must prevent the login from creating a new session.
    const racer = await f.account()
    let releaseLock, lockReady
    const gate = new Promise(resolve => { releaseLock = resolve })
    const ready = new Promise(resolve => { lockReady = resolve })
    const locked = f.sql.begin(async tx => {
      await tx`SELECT id FROM users WHERE id=${racer.id} FOR UPDATE`
      lockReady()
      await gate
      await tx`UPDATE users SET status='suspended' WHERE id=${racer.id}`
    })
    await ready
    const racingLogin = fetch(f.base + '/api/auth/login', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ email: racer.email, password: racer.password }) })
    let loginWaiting = false
    try {
      for (let attempt = 0; attempt < 100; attempt++) {
        const [state] = await f.sql`SELECT exists(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query ILIKE '%for update%') AS waiting`
        if (state.waiting) { loginWaiting = true; break }
        await new Promise(resolve => setTimeout(resolve, 20))
      }
    } finally { releaseLock(); await locked }
    assert.ok(loginWaiting, 'login reached the locked session-issuance recheck')
    assert.equal((await racingLogin).status, 403)
    const [sessionCount] = await f.sql`SELECT count(*)::int AS count FROM auth_sessions WHERE user_id=${racer.id}`
    assert.equal(sessionCount.count, 1, 'only the fixture session exists; racing login issued none')
    // Two active administrators cannot suspend one another concurrently and leave none active.
    const adminA = await f.account('admin'), adminB = await f.account('admin')
    const attempts = await Promise.all([[adminA, adminB], [adminB, adminA]].map(([actor, target]) => fetch(f.base + `/api/admin/users/${target.id}/status`, { method: 'PUT', headers: { cookie: `attackbox_session=${actor.token}`, 'content-type': 'application/json' }, body: JSON.stringify({ suspended: true }) })))
    assert.equal(attempts.filter(r => r.status === 200).length, 1)
    assert.ok(attempts.every(r => [200, 401, 403].includes(r.status)))
    const [admins] = await f.sql`SELECT count(*)::int AS count FROM users WHERE id IN (${adminA.id},${adminB.id}) AND status='active'`
    assert.equal(admins.count, 1)
    // Verify UI controls against real APIs, without replacing browser responses.
    browser = await chromium.launch({ executablePath: '/usr/bin/chromium', args: ['--no-sandbox'] })
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
    await context.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
    const page = await context.newPage(), errors = []
    page.on('pageerror', e => errors.push(e.message))
    await page.goto(f.base + '/admin/users')
    await page.getByRole('heading', { name: '用户管理', exact: true }).waitFor()
    await page.getByRole('textbox', { name: '搜索用户' }).fill(bob.email)
    await page.getByRole('button', { name: '搜索', exact: true }).click()
    await page.waitForFunction(email => document.querySelectorAll('tbody tr').length === 1 && document.querySelector('tbody')?.textContent.includes(email), bob.email)
    await page.getByRole('link', { name: '查看', exact: true }).click()
    await page.getByRole('heading', { name: '账号', exact: true }).waitFor()
    await page.getByRole('button', { name: '停用账号', exact: true }).click()
    await page.getByRole('alertdialog').getByRole('button',{name:'停用账号',exact:true}).click()
    await page.getByRole('button', { name: '恢复账号', exact: true }).waitFor()
    assert.equal((await f.request(`/api/admin/users/${bob.id}`, { actor: f.admin })).data.user.status, 'suspended')
    await page.screenshot({ path: '/tmp/attackbox-user-detail.png', fullPage: true })
    await page.getByRole('button', { name: '恢复账号', exact: true }).click()
    await page.getByRole('alertdialog').getByRole('button',{name:'恢复账号',exact:true}).click()
    await page.getByRole('button', { name: '停用账号', exact: true }).waitFor()
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false)
    await page.goto(f.base + `/admin/users/${alice.id}`)
    await page.getByRole('heading', { name: '学习记录', exact: true }).waitFor()
    assert.equal(await page.locator('progress').getAttribute('value'), '1')
    await page.getByRole('button', { name: /用户菜单/ }).click()
    await page.getByRole('button', { name: '深色外观', exact: true }).click()
    await page.keyboard.press('Escape')
    await page.screenshot({ path: '/tmp/attackbox-user-detail-dark.png', fullPage: true })
    assert.deepEqual(errors, [])
  } finally { await browser?.close(); await worker?.close(); await f.cleanup() }
})
