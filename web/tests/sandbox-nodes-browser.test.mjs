import test from 'node:test'
import assert from 'node:assert/strict'
import { once } from 'node:events'
import { existsSync, mkdirSync } from 'node:fs'
import WebSocket from 'ws'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'

function message(socket) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => { socket.off('message', onMessage); reject(new Error('Worker acknowledgement timeout')) }, 5000)
    function onMessage(raw) { clearTimeout(timer); resolve(JSON.parse(raw.toString())) }
    socket.once('message', onMessage)
  })
}
test('node control: handshake auth, Go-compatible register/heartbeat, replacement connection and administrator UI', { skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.CONTENT_TEST_BASE_URL || !process.env.NODE_TEST_CLUSTER_SECRET, timeout: 60000 }, async () => {
  const f = await learningFixture(), sockets = []
  const url = f.base.replace('http:', 'ws:') + '/cluster/connect'
  let browser
  try {
    await new Promise((resolve, reject) => {
      const s = new WebSocket(url, { headers: { 'X-Node-ID': 'test-node', Authorization: 'Bearer invalid' } })
      s.on('unexpected-response', (_, res) => { res.resume(); assert.equal(res.statusCode, 401); resolve() })
      s.on('open', () => { s.close(); reject(new Error('Unauthenticated worker accepted')) }); s.on('error', reject)
    })
    const worker = new WebSocket(url, { headers: { 'X-Node-ID': 'test-node', Authorization: `Bearer ${process.env.NODE_TEST_CLUSTER_SECRET}` } }); sockets.push(worker)
    await once(worker, 'open')
    const send = (socket, type, payload) => socket.send(JSON.stringify({ id: 'compat-message', type, payload }))
    const register = { node_id: 'test-node', boot_id: 'browser-test-boot', version: '2.0', data_port: 18443, public_data_url: 'ws://127.0.0.1:18443', capabilities: { cpu_cores: 4, total_mem_mb: 8192, os_version: 'linux/amd64', has_kvm: false } }
    let ack = message(worker); send(worker, 'register', register)
    assert.equal((await ack).payload.public_key.length, 64)
    const heartbeat = { node_id: 'test-node', boot_id: register.boot_id, seq: 1, inventory_revision: 2, timestamp: Math.floor(Date.now() / 1000), load_avg: 0.42, mem_available_mb: 4096, disk_free_gb: 80, active_sandboxes: 1, sandboxes: [{ id: 'sbx_protocol_test', state: 'running', last_active_at: Math.floor(Date.now() / 1000), gui_running: false }] }
    ack = message(worker); send(worker, 'heartbeat', heartbeat); assert.equal((await ack).type, 'heartbeat_ack')
    const anon = await fetch(f.base + '/api/admin/nodes'); assert.equal(anon.status, 401)
    const user = await f.account(); await f.request('/api/admin/nodes', { actor: user, status: 403 })
    const observed = await f.request('/api/admin/nodes', { actor: f.admin })
    assert.equal(observed.data.nodes[0].state, 'online'); assert.equal(observed.data.schedulingEnabled, false)
    assert.equal(observed.response.headers.get('cache-control'), 'private, no-store')
    assert.ok(!JSON.stringify(observed.data).includes(process.env.NODE_TEST_CLUSTER_SECRET))
    browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH || (existsSync('/usr/bin/chromium') ? '/usr/bin/chromium' : undefined), args: ['--no-sandbox'] })
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
    await context.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
    const page = await context.newPage(), errors = []; page.on('pageerror', e => errors.push(e.message))
    await page.goto(f.base + '/admin/nodes'); await page.locator('.node-state.online').waitFor()
    assert.ok((await page.locator('.node-metrics').innerText()).includes('4.0 GB / 8.0 GB'))
    await page.locator('.inventory summary').click(); await page.getByText('sbx_protocol_test', { exact: true }).waitFor()
    assert.equal(await page.getByRole('link', { name: '节点管理', exact: true }).count(), 1)
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    if (process.env.NODE_SCREENSHOT_DIR) { mkdirSync(process.env.NODE_SCREENSHOT_DIR, { recursive: true }); await page.screenshot({ path: process.env.NODE_SCREENSHOT_DIR + '/nodes-online.png', fullPage: true }) }
    // Same boot, replacement socket: an older clock is ignored, a newer clock wins.
    const replacement = new WebSocket(url, { headers: { 'X-Node-ID': 'test-node', Authorization: `Bearer ${process.env.NODE_TEST_CLUSTER_SECRET}` } }); sockets.push(replacement)
    await once(replacement, 'open'); ack = message(replacement); send(replacement, 'register', register); await ack
    ack = message(replacement); send(replacement, 'heartbeat', { ...heartbeat, seq: 2 }); await ack
    const closed = once(replacement, 'close'); replacement.close(); await closed
    await page.getByRole('button', { name: '刷新', exact: true }).click(); await page.locator('.node-state.offline').waitFor()
    await page.getByText('历史数据', { exact: true }).waitFor()
    assert.deepEqual(errors, [])
    await context.close()
  } finally {
    for (const socket of sockets) socket.terminate()
    if (browser) await browser.close()
    await f.cleanup()
  }
})
