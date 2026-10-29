// Dedicated-host recovery acceptance. Uses only the explicitly named test units.
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { once } from 'node:events'
import WebSocket from 'ws'
if (process.env.RUN_REAL_SANDBOX_ACCEPTANCE !== 'true' || !process.env.SANDBOX_ACCEPTANCE_FIXTURE)
  throw Error('Explicit real acceptance opt-in required')
const actors = JSON.parse(readFileSync(process.env.SANDBOX_ACCEPTANCE_FIXTURE)),
  base = 'http://127.0.0.1:3031'
const remote = command =>
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
      command
    ],
    { timeout: 30000, stdio: 'pipe' }
  )
async function req(path, actor = actors.admin, method = 'GET', body, status = 200) {
  const r = await fetch(base + path, {
      method,
      headers: {
        cookie: `attackbox_session=${actor.token}`,
        ...(body ? { 'content-type': 'application/json' } : {})
      },
      body: body ? JSON.stringify(body) : undefined,
      signal: AbortSignal.timeout(45000)
    }),
    j = await r.json()
  assert.equal(r.status, status, `${path}: ${j.message || 'unexpected status'}`)
  return j.data
}
async function until(check, timeout = 60000) {
  const end = Date.now() + timeout
  while (Date.now() < end) {
    try {
      if (await check()) return
    } catch {
      /* 清理失败不影响断言结果 */
    }
    await new Promise(r => setTimeout(r, 500))
  }
  throw Error('Recovery acceptance timed out')
}
const node = async () => (await req('/api/admin/nodes')).nodes.find(n => n.id === 'acceptance-node')
async function settings(patch) {
  const current = await node()
  return req('/api/admin/nodes/acceptance-node/settings', actors.admin, 'PUT', {
    ...current.settings,
    ...patch
  })
}
const start = actor => req('/api/sandbox/start', actor, 'POST')
const state = actor => req('/api/sandbox/status', actor)
const empty = async () => {
  const n = await node()
  return n?.state === 'online' && !n.sandboxes.length
}
async function stop(actor) {
  const s = (await state(actor)).instance
  if (s) await req('/api/sandbox/stop', actor, 'POST', { sandboxId: s.id })
}
let paused = false,
  activity,
  socket
try {
  await stop(actors.alice)
  await stop(actors.bob)
  await until(empty)
  await settings({
    draining: false,
    memoryMb: 512,
    pidsMax: 128,
    ttlSeconds: 180,
    idleSeconds: 60,
    maxSlots: 1,
    label: '验收策略持久化'
  })
  const first = await start(actors.alice)
  assert.equal(first.policy.resources.memory_max_mb, 512)
  assert.equal(first.policy.resources.pids_max, 128)
  const quota = remote(
    `sudo -n cat /sys/fs/cgroup/attackbox-acceptance-20261024/${first.id}/memory.max`
  )
    .toString()
    .trim()
  assert.equal(quota, '536870912')
  await settings({ draining: true, memoryMb: 768 })
  assert.equal((await start(actors.alice)).id, first.id)
  await req('/api/sandbox/start', actors.bob, 'POST', undefined, 503)
  assert.equal((await state(actors.alice)).instance.policy.resources.memory_max_mb, 512)
  console.log('PASS draining preserves live instance and policy changes affect only future creates')
  remote('sudo -n systemctl restart attackbox-acceptance-web')
  await until(async () => {
    const n = await node()
    return n?.state === 'online' && n.settings.draining && n.settings.memoryMb === 768
  })
  await until(empty)
  assert.equal((await state(actors.alice)).instance, null)
  console.log('PASS Control restart reloads settings and cleans unknown real Worker instances')
  await settings({ draining: false })
  const before = await node()
  await start(actors.alice)
  remote('sudo -n systemctl restart attackbox-acceptance-node')
  await until(async () => {
    const n = await node()
    return n?.state === 'online' && n.bootId !== before.bootId && !n.sandboxes.length
  })
  assert.equal((await state(actors.alice)).instance, null)
  console.log('PASS Worker restart resets real snapshot/process/cgroup lifecycle and reconnects')
  remote('sudo -n systemctl kill --kill-who=main --signal=STOP attackbox-acceptance-node')
  paused = true
  await req('/api/sandbox/start', actors.alice, 'POST', undefined, 502)
  const pending = (await state(actors.alice)).instance
  assert.ok(pending && pending.state === 'cleanup_failed')
  remote('sudo -n systemctl kill --kill-who=main --signal=CONT attackbox-acceptance-node')
  paused = false
  await until(async () => !(await state(actors.alice)).instance && (await empty()))
  console.log(
    'PASS unresponsive real Worker retains uncertain ownership, then converges after recovery'
  )
  await settings({ maxSlots: 2, memoryMb: 256, pidsMax: 64, ttlSeconds: 60, idleSeconds: 60 })
  const ttl = await start(actors.alice),
    issued = await req('/api/sandbox/ticket', actors.alice, 'POST', { sandboxId: ttl.id })
  const url = new URL(issued.url)
  url.searchParams.set('ticket', issued.ticket)
  url.searchParams.set('win', 'ttl-activity')
  socket = new WebSocket(url, { origin: base })
  socket.on('error', () => {})
  await once(socket, 'open')
  activity = setInterval(() => {
    if (socket.readyState === WebSocket.OPEN) socket.send(Buffer.from('true\r'))
  }, 7000)
  await settings({ ttlSeconds: 180, idleSeconds: 60 })
  await start(actors.bob)
  const began = Date.now()
  console.log(
    'WAIT real TTL with active terminal input, and Idle with API polling only (about 60 seconds)'
  )
  await until(
    async () => !(await state(actors.alice)).instance && !(await state(actors.bob)).instance,
    85000
  )
  assert.ok(Date.now() - began >= 55000)
  assert.ok(Date.parse(ttl.expiresAt) - Date.parse(ttl.startedAt) <= 65000)
  await until(empty)
  console.log('PASS active input cannot extend TTL; API polling does not extend Idle')
} finally {
  clearInterval(activity)
  socket?.close()
  if (paused)
    remote('sudo -n systemctl kill --kill-who=main --signal=CONT attackbox-acceptance-node')
}
