import test from 'node:test'
import assert from 'node:assert/strict'
import { once } from 'node:events'
import { existsSync, mkdirSync, mkdtempSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { chromium } from 'playwright'
import WebSocket from 'ws'
import { learningFixture } from './helpers/learning-fixture.mjs'
import { sandboxWorker } from './helpers/sandbox-worker.mjs'
const waitUntil = async callback => { for (let i = 0; i < 100; i++) { if (await callback()) return; await new Promise(r => setTimeout(r, 100)) } throw new Error('Condition timed out') }
test('sandbox frontend/backend: stable identity, concurrent start, ticket security, terminal lifecycle and administrator release', { skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.CONTENT_TEST_BASE_URL || !process.env.NODE_TEST_CLUSTER_SECRET, timeout: 120000 }, async () => {
  const f = await learningFixture()
  let worker, browser, debugPage
  const browserErrors = []
  try {
    worker = await sandboxWorker(f.base, process.env.NODE_TEST_CLUSTER_SECRET)
    const alice = await f.account(), bob = await f.account(), mod = await f.createModule()
    const first = await Promise.all([1, 2, 3].map(() => f.request('/api/sandbox/start', { actor: alice, method: 'POST', body: { userId: bob.id, policy: { memory_max_mb: 999999 }, slot: 250 } })))
    const id = first[0].data.id
    assert.ok(first.every(r => r.data.id === id)); assert.equal(worker.requests.filter(r => r.type === 'create_box').length, 1)
    const numericIdentity = (await f.sql`SELECT worker_user_id FROM sandbox_identities WHERE user_id=${alice.id}`)[0].worker_user_id
    assert.equal(String(worker.boxes.get(id).user_id), String(numericIdentity)); assert.equal(worker.boxes.get(id).policy.resources.memory_max_mb, 2048)
    assert.equal((await f.request('/api/sandbox/status', { actor: bob })).data.instance, null)
    await f.request('/api/sandbox/ticket', { actor: bob, method: 'POST', body: { sandboxId: id }, status: 404 })
    await f.request('/api/sandbox/stop', { actor: bob, method: 'POST', body: { sandboxId: id }, status: 404 })
    await f.request('/api/admin/instances', { actor: bob, status: 403 })
    await f.request('/api/sandbox/start', { method: 'POST', status: 401 })
    const csrf = await fetch(f.base + '/api/sandbox/stop', { method: 'POST', headers: { cookie: `attackbox_session=${alice.token}`, origin: 'https://other.example', 'content-type': 'application/json' }, body: JSON.stringify({ sandboxId: id }) }); assert.equal(csrf.status, 403)
    const ticket = (await f.request('/api/sandbox/ticket', { actor: alice, method: 'POST', body: { sandboxId: id } })).data
    if (process.env.GO_TEST_BINARY) {
      const folder = mkdtempSync(path.join(tmpdir(), 'nuxt-ticket-')), file = path.join(folder, 'ticket.json')
      try {
        writeFileSync(file, JSON.stringify({ ticket: ticket.ticket, publicKey: worker.publicKey, sandboxId: id, workerUserId: Number(numericIdentity) }), { mode: 0o600 })
        execFileSync(process.env.GO_TEST_BINARY, ['test', './protocol', '-run', 'TestNuxtTicketCompatibility', '-count=1'], { cwd: path.resolve(import.meta.dirname, '../..'), env: { ...process.env, NUXT_TICKET_TEST_FILE: file }, timeout: 30000, stdio: 'pipe' })
      } finally { rmSync(folder, { recursive: true, force: true }) }
    }
    async function connectToken() { const s = new WebSocket(ticket.url + '?ticket=' + encodeURIComponent(ticket.ticket)); await once(s, 'open'); return s }
    const consumed = await connectToken(); consumed.close()
    await new Promise((resolve, reject) => { const s = new WebSocket(ticket.url + '?ticket=' + encodeURIComponent(ticket.ticket)); s.on('unexpected-response', (_, r) => { r.resume(); assert.equal(r.statusCode, 403); resolve() }); s.on('open', () => { s.close(); reject(new Error('Ticket replay accepted')) }); s.on('error', reject) })
    browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH || (existsSync('/usr/bin/chromium') ? '/usr/bin/chromium' : undefined), args: ['--no-sandbox'] })
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } }); await context.addCookies([{ name: 'attackbox_session', value: alice.token, url: f.base }])
    const page = await context.newPage(), errors = browserErrors; debugPage = page; page.on('pageerror', e => errors.push(e.message)); page.setDefaultTimeout(15000)
    await page.goto(`${f.base}/learn/modules/${mod.module.slug}/intro`)
    await page.getByRole('button', { name: 'CLI 终端', exact: true }).click()
    await page.locator('.terminal-status').filter({ hasText: '已连接' }).waitFor()
    const input = page.locator('.xterm-helper-textarea').first(); await input.focus(); await input.pressSequentially('whoami'); await input.press('Enter')
    await waitUntil(() => worker.commands.includes('whoami'))
    await waitUntil(async () => (await page.locator('.xterm-rows').innerText()).includes('Executed: whoami'))
    const opened = worker.terminals.length
    await page.getByRole('button', { name: '收起练习面板', exact: true }).click(); await page.getByRole('button', { name: '恢复练习面板', exact: true }).click()
    await page.getByRole('button', { name: '关闭练习面板', exact: true }).click(); await page.getByRole('button', { name: 'CLI 终端', exact: true }).click()
    assert.equal(worker.terminals.length, opened, 'hide/minimize preserve existing terminal connection')
    await page.getByRole('button', { name: '新建终端', exact: true }).click(); await waitUntil(() => worker.terminals.length === opened + 1)
    assert.equal(new Set(worker.terminals.slice(1).map(s => s.windowId)).size, 2, 'separate PTYs use distinct window IDs')
    await page.getByRole('button', { name: 'GUI 桌面', exact: true }).click(); await page.getByRole('heading', { name: '桌面接入将在下一阶段开放' }).waitFor()
    await page.getByRole('button', { name: 'CLI 终端', exact: true }).click()
    assert.equal(worker.terminals.length, opened + 1, 'CLI/GUI mode changes preserve existing terminal connections')
    const beforeReconnect = worker.tickets.length
    worker.terminals.at(-1).close(1011, 'Test interruption')
    await waitUntil(() => worker.tickets.length > beforeReconnect)
    await page.locator('.terminal-status:visible').filter({ hasText: '已连接' }).waitFor()
    assert.equal(worker.requests.filter(r => r.type === 'create_box').length, 1, 'reconnect and mode changes never create a second sandbox')
    if (process.env.SANDBOX_SCREENSHOT_DIR) { mkdirSync(process.env.SANDBOX_SCREENSHOT_DIR, { recursive: true }); await page.screenshot({ path: process.env.SANDBOX_SCREENSHOT_DIR + '/learning-terminal.png' }) }
    const admin = await browser.newContext({ viewport: { width: 1600, height: 1000 } }); await admin.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
    const adminPage = await admin.newPage(); adminPage.on('pageerror', e => errors.push(e.message)); await adminPage.goto(f.base + '/admin/instances')
    const row = adminPage.locator('tbody tr').filter({ hasText: id }); await row.waitFor(); assert.ok((await row.innerText()).includes(alice.email))
    if (process.env.SANDBOX_SCREENSHOT_DIR) await adminPage.screenshot({ path: process.env.SANDBOX_SCREENSHOT_DIR + '/admin-instances.png', fullPage: true })
    adminPage.once('dialog', dialog => dialog.accept()); await row.getByRole('button', { name: '释放', exact: true }).click(); await row.waitFor({ state: 'hidden' })
    assert.equal((await f.request('/api/sandbox/status', { actor: alice })).data.instance, null)
    await page.bringToFront(); await page.getByRole('button', { name: '启动环境', exact: true }).waitFor()
    await page.getByRole('button', { name: '启动环境', exact: true }).click(); await page.locator('.terminal-status:visible').filter({ hasText: '已连接' }).waitFor()
    assert.equal(String([...worker.boxes.values()][0].user_id), String(numericIdentity), 'restart keeps the durable Worker identity')
    page.once('dialog', dialog => dialog.accept()); await page.getByRole('button', { name: '释放环境', exact: true }).click(); await page.getByRole('button', { name: '启动环境', exact: true }).waitFor()
    assert.equal(worker.boxes.size, 0); assert.deepEqual(errors, [])
    await admin.close(); await context.close()
  } catch (error) {
    if (debugPage && !debugPage.isClosed()) {
      console.error((await debugPage.locator('body').innerText()).slice(-2500), browserErrors)
      if (process.env.SANDBOX_SCREENSHOT_DIR) { mkdirSync(process.env.SANDBOX_SCREENSHOT_DIR, { recursive: true }); await debugPage.screenshot({ path: process.env.SANDBOX_SCREENSHOT_DIR + '/failure.png' }) }
    }
    throw error
  } finally { if (browser) await browser.close(); if (worker) await worker.close(); await f.cleanup() }
})
