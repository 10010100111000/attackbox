// Explicit real-Worker acceptance only; never uses the simulated Worker fixture.
import assert from 'node:assert/strict'
import { readFileSync, writeFileSync } from 'node:fs'
import { once } from 'node:events'
import { execFileSync } from 'node:child_process'
import WebSocket from 'ws'
import { chromium } from 'playwright'
const base = 'http://127.0.0.1:3031'
if (process.env.RUN_REAL_SANDBOX_ACCEPTANCE !== 'true' || !process.env.SANDBOX_ACCEPTANCE_FIXTURE)
  throw Error('Explicit real acceptance opt-in and dedicated fixture required')
const file = process.env.SANDBOX_ACCEPTANCE_FIXTURE,
  actors = JSON.parse(readFileSync(file, 'utf8'))
const checkpoint = () => writeFileSync(file, JSON.stringify(actors), { mode: 0o600 })
async function request(path, actor = actors.alice, method = 'GET', body, expected = 200) {
  const response = await fetch(base + path, {
    method,
    headers: {
      cookie: `attackbox_session=${actor.token}`,
      ...(body ? { 'content-type': 'application/json' } : {})
    },
    body: body ? JSON.stringify(body) : undefined
  })
  const json = await response.json()
  assert.equal(
    response.status,
    expected,
    `${path}: ${json.message || json.statusMessage || 'unexpected response'}`
  )
  return json.data
}
async function waitUntil(check, limit = 30000) {
  const deadline = Date.now() + limit
  while (Date.now() < deadline) {
    if (await check()) return
    await new Promise(r => setTimeout(r, 250))
  }
  throw Error('Acceptance condition timed out')
}
async function terminal(id, windowId) {
  const issued = await request('/api/sandbox/ticket', actors.alice, 'POST', { sandboxId: id })
  const url = new URL(issued.url)
  url.searchParams.set('ticket', issued.ticket)
  url.searchParams.set('win', windowId)
  const socket = new WebSocket(url, { origin: base })
  let output = ''
  socket.on('message', data => (output += data.toString()))
  await once(socket, 'open')
  return {
    socket,
    get output() {
      return output
    },
    async command(command, marker) {
      socket.send(Buffer.from(command + '\r'))
      await waitUntil(() => output.includes(marker), 20000)
    }
  }
}
let browser
const terminals = []
try {
  const nodes = await request('/api/admin/nodes', actors.admin)
  assert.ok(JSON.stringify(nodes).includes('acceptance-node'))
  const config = nodes.nodes.find(n => n.id === 'acceptance-node')
  assert.ok(config, 'Dedicated acceptance node required')
  await request('/api/admin/nodes/acceptance-node/settings', actors.admin, 'PUT', {
    ...config.settings,
    draining: false,
    maxSlots: 1,
    memoryMb: 1024,
    pidsMax: 512,
    ttlSeconds: 300,
    idleSeconds: 120
  })
  const old = (await request('/api/sandbox/status')).instance
  if (old) await request('/api/sandbox/stop', actors.alice, 'POST', { sandboxId: old.id })
  await waitUntil(async () => {
    const state = await request('/api/admin/nodes', actors.admin)
    const node = state.nodes.find(n => n.id === 'acceptance-node')
    return node?.state === 'online' && !node.sandboxes.length
  }, 60000)
  const first = await request('/api/sandbox/start', actors.alice, 'POST')
  actors.instance = first.id
  checkpoint()
  assert.equal(first.state, 'running')
  const duplicate = await request('/api/sandbox/start', actors.alice, 'POST')
  assert.equal(duplicate.id, first.id)
  await request('/api/sandbox/start', actors.bob, 'POST', undefined, 503)
  await request('/api/sandbox/ticket', actors.bob, 'POST', { sandboxId: first.id }, 404)
  console.log('PASS real create, same-user reuse, concurrency=1 and ownership')
  const a = await terminal(first.id, 'acceptance-a')
  terminals.push(a)
  await a.command(
    "printf 'ACCEPTANCE-FILE-OK\\n' > /home/box/acceptance.txt; id; cat /home/box/acceptance.txt",
    'ACCEPTANCE-FILE-OK\r\n'
  )
  assert.match(a.output, /uid=1001\(box\)/)
  const b = await terminal(first.id, 'acceptance-b')
  terminals.push(b)
  await b.command("cat /home/box/acceptance.txt; printf 'SECOND-SHELL-%s\\n' OK", 'SECOND-SHELL-OK')
  console.log('PASS real PTY identity, file write and second-terminal shared filesystem')
  const before = (await request('/api/admin/nodes', actors.admin)).nodes[0]
  execFileSync(
    'ssh',
    [
      '-F',
      '/dev/null',
      '-p',
      '15222',
      '-o',
      'HostKeyAlias=100.69.228.63',
      '-o',
      'BatchMode=yes',
      '-o',
      'StrictHostKeyChecking=yes',
      '-o',
      'UserKnownHostsFile=/tmp/attackbox-ssh-known-hosts',
      'laji@127.0.0.1',
      'sudo -n ss -K dst 127.0.0.1 dport = :3031'
    ],
    { timeout: 15000, stdio: 'pipe' }
  )
  await waitUntil(async () => {
    try {
      const n = (await request('/api/admin/nodes', actors.admin)).nodes[0]
      return (
        n.state === 'online' &&
        n.bootId === before.bootId &&
        n.lastHeartbeatAt > before.lastHeartbeatAt
      )
    } catch {
      return false
    }
  }, 60000)
  assert.equal((await request('/api/sandbox/status')).instance.id, first.id)
  await b.command(
    "cat /home/box/acceptance.txt; printf 'CONTROL-RECONNECT-%s\\n' OK",
    'CONTROL-RECONNECT-OK'
  )
  console.log('PASS real control TCP disconnect reconnects with the same boot and live instance')

  browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
    args: ['--no-sandbox']
  })
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  await context.addCookies([{ name: 'attackbox_session', value: actors.alice.token, url: base }])
  const page = await context.newPage()
  page.setDefaultTimeout(45000)
  await page.goto(
    actors.module ? `${base}/learn/modules/${actors.module.module.slug}/runtime` : `${base}/learn`
  )
  await page.waitForFunction(() => Boolean(document.querySelector('#__nuxt')?.__vue_app__))
  await page.goto(`${base}/gui`)
  await page.getByText('桌面已连接', { exact: true }).waitFor({ timeout: 60000 })
  const canvas = page.locator('.desktop-host canvas')
  await canvas.waitFor()
  assert.ok(await canvas.evaluate(c => c.width > 0 && c.height > 0))
  await page.screenshot({ path: '/tmp/attackbox-real-gui.png', fullPage: true })
  assert.equal((await request('/api/sandbox/status')).instance.id, first.id)
  await a.command(
    "DISPLAY=:1 xterm -geometry 90x22+60+60 -title AcceptanceInput >/tmp/acceptance-terminal.log 2>&1 & sleep 2; printf 'DESKTOP-TERMINAL-%s\\n' READY",
    'DESKTOP-TERMINAL-READY'
  )
  await canvas.click({ position: { x: 150, y: 150 } })
  await canvas.focus()
  console.log('Desktop input focus:', await canvas.evaluate(c => document.activeElement === c))
  await page.keyboard.type("printf 'GUI-SHARED-OK\\n' > /home/box/gui-acceptance.txt", {
    delay: 15
  })
  await page.keyboard.press('Enter')
  await b.command(
    "for i in $(seq 1 50); do if test -f /home/box/gui-acceptance.txt; then cat /home/box/gui-acceptance.txt; printf 'GUI-INPUT-%s\\n' VERIFIED; break; fi; sleep 0.2; done",
    'GUI-INPUT-VERIFIED'
  )
  assert.ok(b.output.includes('GUI-SHARED-OK'))
  await page.screenshot({ path: '/tmp/attackbox-real-gui.png', fullPage: true })
  console.log('PASS real noVNC RFB desktop, pointer/keyboard and GUI/CLI shared home')
  await b.command(
    "pgrep -x Xtigervnc > /home/box/gui-pid-before; printf 'GUI-PID-%s\\n' SAVED",
    'GUI-PID-SAVED'
  )
  await page.reload()
  await page.getByRole('button', { name: 'GUI 桌面', exact: true }).click()
  await page.getByText('桌面已连接', { exact: true }).waitFor({ timeout: 60000 })
  await b.command(
    'test "$(pgrep -x Xtigervnc)" = "$(cat /home/box/gui-pid-before)" && printf \'GUI-REUSED-%s\\n\' OK',
    'GUI-REUSED-OK'
  )
  const c = await terminal(first.id, 'after-reconnect')
  terminals.push(c)
  await c.command("cat /home/box/gui-acceptance.txt; printf 'CLI-REOPEN-%s\\n' OK", 'CLI-REOPEN-OK')
  console.log(
    'PASS browser reload reuses the existing VNC process; a new CLI shell retains home files'
  )

  await browser.close()
  browser = undefined
  for (const t of terminals) t.socket.close()
  await request('/api/sandbox/stop', actors.alice, 'POST', { sandboxId: first.id })
  await waitUntil(async () => !(await request('/api/sandbox/status')).instance)
  actors.instance = null
  checkpoint()
  await waitUntil(
    async () => !(await request('/api/admin/nodes', actors.admin)).nodes[0].sandboxes.length
  )
  const adminRelease = await request('/api/sandbox/start', actors.bob, 'POST')
  await request(`/api/admin/instances/${adminRelease.id}/stop`, actors.admin, 'POST')
  assert.equal((await request('/api/sandbox/status', actors.bob)).instance, null)
  console.log(
    'PASS real user and administrator release acknowledged; inspect host cleanup separately'
  )
} catch (error) {
  if (browser) {
    const pages = browser.contexts().flatMap(c => c.pages())
    await pages[0]
      ?.screenshot({ path: '/tmp/attackbox-real-failure.png', fullPage: true })
      .catch(() => {})
  }
  throw error
} finally {
  for (const t of terminals) t.socket.close()
  await browser?.close()
}
