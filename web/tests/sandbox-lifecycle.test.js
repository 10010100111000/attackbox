import test from 'node:test'
import assert from 'node:assert/strict'
import { createPublicKey, verify } from 'node:crypto'
import { sandboxConfig } from '../server/sandbox/config.ts'
import { SandboxHub } from '../server/sandbox/hub.ts'
import { SandboxController } from '../server/sandbox/controller.ts'
const flush = () => new Promise(resolve => setImmediate(resolve))
function fixture({ scheduling = true, timeout = 1000 } = {}) {
  let now = 1700000000000, revision = 0, seq = 0, boot = 'boot-1'
  const config = sandboxConfig({ SANDBOX_CONTROL_ENABLED: 'true', SANDBOX_SCHEDULING_ENABLED: String(scheduling), SANDBOX_CLUSTER_SECRET: 'test-secret-000000000000000000000000', SANDBOX_SIGNING_SEED_HEX: '11'.repeat(32), SANDBOX_NODES_JSON: '[{"id":"test-node","label":"Test","publicDataUrl":"ws://127.0.0.1:18443","maxSlots":2}]' })
  const hub = new SandboxHub(config, () => now), controller = new SandboxController(hub, () => now, timeout)
  let peer
  const sent = []
  function input(type, payload, id = 'worker-message') { hub.message(peer, JSON.stringify({ id, type, payload })) }
  function connect(newBoot = boot) {
    boot = newBoot; seq = 0; peer = { id: 'peer-' + Math.random(), send: raw => sent.push(JSON.parse(raw)), close() {} }
    hub.open(peer, 'test-node')
    input('register', { node_id: 'test-node', boot_id: boot, version: '2.0', data_port: 18443, public_data_url: 'ws://127.0.0.1:18443', capabilities: { cpu_cores: 4, total_mem_mb: 16384, os_version: 'linux/amd64', has_kvm: false } })
  }
  function heartbeat(boxes = [], rev = revision) {
    revision = rev
    input('heartbeat', { node_id: 'test-node', boot_id: boot, seq: ++seq, inventory_revision: revision, timestamp: Math.floor(now / 1000), load_avg: 0.2, mem_available_mb: 8192, disk_free_gb: 80, active_sandboxes: boxes.length, sandboxes: boxes.map(box => typeof box === 'string' ? { id: box, state: 'running', last_active_at: Math.floor(now / 1000) } : box) })
  }
  const requests = type => sent.filter(m => m.type === type)
  function respond(request, payload = {}) { input(request.type + '_resp', { success: true, sandbox_id: request.payload.sandbox_id, boot_id: request.payload.boot_id, ...payload }, request.id) }
  async function started(user = 'user-1', workerId = 1) {
    const task = controller.start(user, workerId), request = requests('create_box').at(-1)
    respond(request, { inventory_revision: ++revision, expires_at: Math.floor(now / 1000) + 7200, sandbox_ip: '10.200.0.2' })
    return await task
  }
  connect(); heartbeat()
  return { config, hub, controller, sent, requests, respond, started, heartbeat, connect, advance: ms => now += ms, input, close: () => hub.close(peer) }
}
test('concurrent starts reserve one instance, authoritative policy and one numeric identity', async () => {
  const f = fixture(), first = f.controller.start('user-1', 42), duplicate = await f.controller.start('user-1', 42)
  assert.equal(duplicate.state, 'creating'); assert.equal(f.requests('create_box').length, 1)
  const request = f.requests('create_box')[0]
  assert.equal(request.payload.user_id, 42); assert.equal(request.payload.policy.resources.memory_max_mb, 2048)
  f.heartbeat([], 0); assert.equal(f.controller.status('user-1').instance.id, duplicate.id)
  f.respond(request, { inventory_revision: 1, expires_at: 1700007200, sandbox_ip: '10.200.0.2' })
  const running = await first; assert.equal(running.id, duplicate.id)
  assert.equal((await f.controller.start('user-1', 42)).id, running.id)
  const stop = f.controller.stopOwned('user-1', running.id); f.respond(f.requests('destroy_box').at(-1)); await stop
})
test('slot reservations limit concurrency, per-user ownership protects tickets and stop', async () => {
  const f = fixture(), a = await f.started('alice', 10), b = await f.started('bob', 11)
  assert.notEqual(a.slot, b.slot)
  await assert.rejects(f.controller.start('eve', 12), { statusCode: 503 })
  assert.throws(() => f.controller.ticket('eve', a.id), { statusCode: 404 })
  await assert.rejects(f.controller.stopOwned('bob', a.id), { statusCode: 404 })
  const ticket = f.controller.ticket('alice', a.id), [payload, sig] = ticket.ticket.split('.')
  assert.ok(verify(null, Buffer.from(payload, 'base64url'), createPublicKey(f.config.signingKey), Buffer.from(sig, 'base64url')))
  const claims = JSON.parse(Buffer.from(payload, 'base64url'))
  assert.equal(claims.sub, 10); assert.equal(claims.sbx, a.id); assert.equal(claims.typ, 'pty'); assert.equal(claims.exp, 1700000060)
  assert.notEqual(f.controller.ticket('alice', a.id).ticket, ticket.ticket)
  f.hub.shutdown()
})
test('failed creation reserves ownership until a confirmed compensation, and heartbeat retries cleanup', async () => {
  const f = fixture(), task = f.controller.start('alice', 1)
  f.respond(f.requests('create_box').at(-1), { success: false, error: 'partial failure' })
  await assert.rejects(task, { statusCode: 502 }); await flush()
  const id = f.controller.status('alice').instance.id
  f.respond(f.requests('destroy_box').at(-1), { success: false }); await flush()
  assert.equal(f.controller.status('alice').instance.state, 'cleanup_failed')
  assert.equal((await f.controller.start('alice', 1)).id, id)
  f.heartbeat([], 0); assert.equal(f.controller.status('alice').instance.id, id, 'pre-create empty inventory cannot release uncertain resources')
  f.respond(f.requests('destroy_box').at(-1)); await flush()
  assert.equal(f.controller.status('alice').instance, null)
})
test('timed-out create and late success cannot resurrect a stopped instance', async () => {
  const f = fixture({ timeout: 20 }), task = f.controller.start('alice', 1), create = f.requests('create_box').at(-1)
  await assert.rejects(task, { statusCode: 502 })
  f.respond(create, { inventory_revision: 1, expires_at: 1700007200, sandbox_ip: '10.200.0.2' })
  const destroy = f.requests('destroy_box').at(-1); assert.equal(f.requests('destroy_box').length, 1)
  f.respond(destroy); await flush(); assert.equal(f.controller.status('alice').instance, null)
})
test('concurrent release is idempotent, and stop in flight wins over create acknowledgement', async () => {
  const f = fixture(), start = f.controller.start('alice', 1), create = f.requests('create_box').at(-1)
  const id = f.controller.status('alice').instance.id, stop = f.controller.stopOwned('alice', id), duplicate = f.controller.stopOwned('alice', id)
  assert.equal(f.requests('destroy_box').length, 1)
  f.respond(create, { inventory_revision: 1, expires_at: 1700007200, sandbox_ip: '10.200.0.2' })
  assert.equal((await start).state, 'stopping')
  f.respond(f.requests('destroy_box').at(-1)); await Promise.all([stop, duplicate])
  assert.equal(f.controller.status('alice').instance, null)
})
test('old inventories do not delete a confirmed create; newer absent inventory confirms Worker recycling', async () => {
  const f = fixture(), running = await f.started()
  f.heartbeat([], 0); assert.equal(f.controller.status('user-1').instance.id, running.id)
  f.heartbeat([running.id], 1); assert.ok(f.controller.status('user-1').instance.lastActiveAt)
  f.heartbeat([], 2); assert.equal(f.controller.status('user-1').instance, null)
})
test('unknown inventory after Control restart blocks scheduling until cleanup and fresh inventory', async () => {
  const f = fixture(); f.heartbeat(['sbx_unknown'], 1)
  assert.equal(f.requests('destroy_box').length, 1)
  await assert.rejects(f.controller.start('alice', 1), { statusCode: 503 })
  f.respond(f.requests('destroy_box').at(-1)); await flush()
  await assert.rejects(f.controller.start('alice', 1), { statusCode: 503 })
  f.heartbeat([], 2); const running = await f.started('alice', 1); assert.equal(running.state, 'running')
})
test('observation mode never schedules or destroys observed sandboxes', async () => {
  const f = fixture({ scheduling: false }); f.heartbeat(['sbx_unknown'], 1)
  assert.equal(f.requests('destroy_box').length, 0)
  await assert.rejects(f.controller.start('alice', 1), { statusCode: 503 })
})
test('disconnect blocks tickets and retains cleanup ownership; a new boot clears old records', async () => {
  const f = fixture(), running = await f.started()
  f.close(); assert.equal(f.controller.status('user-1').instance.connected, false)
  assert.throws(() => f.controller.ticket('user-1', running.id), { statusCode: 409 })
  await assert.rejects(f.controller.stopOwned('user-1', running.id), { statusCode: 502 })
  assert.equal(f.controller.status('user-1').instance.state, 'cleanup_failed')
  f.connect('boot-2'); f.heartbeat([], 0); assert.equal(f.controller.status('user-1').instance, null)
})
test('duplicate successful acknowledgement does not destroy an active instance; expired instances get no tickets', async () => {
  const f = fixture(), running = await f.started(), create = f.requests('create_box').at(-1)
  f.respond(create, { inventory_revision: 1, expires_at: 1700007200, sandbox_ip: '10.200.0.2' })
  assert.equal(f.requests('destroy_box').length, 0)
  f.advance(7200001); assert.throws(() => f.controller.ticket('user-1', running.id), { statusCode: 409 })
})

test('Worker autonomous stopping state can converge to absent without a Control destroy operation', async () => {
  const f = fixture(), running = await f.started()
  f.heartbeat([{ id: running.id, state: 'stopping', last_active_at: 1700000000 }], 2)
  assert.equal(f.controller.status('user-1').instance.state, 'stopping')
  f.heartbeat([], 3)
  assert.equal(f.controller.status('user-1').instance, null)
})

test('GUI ticket binds vnc to the same owned instance without another create', async () => {
  const f = fixture(), running = await f.started('alice', 8)
  const ticket = f.controller.ticket('alice', running.id, 'gui')
  assert.equal(ticket.url, 'ws://127.0.0.1:18443/ws/gui')
  const [payload, signature] = ticket.ticket.split('.')
  assert.ok(verify(null, Buffer.from(payload, 'base64url'), createPublicKey(f.config.signingKey), Buffer.from(signature, 'base64url')))
  const claims = JSON.parse(Buffer.from(payload, 'base64url'))
  assert.equal(claims.typ, 'vnc'); assert.equal(claims.sub, 8); assert.equal(claims.sbx, running.id)
  assert.equal(f.requests('create_box').length, 1)
  assert.throws(() => f.controller.ticket('bob', running.id, 'gui'), { statusCode: 404 })
})
test('draining preserves existing instances and new per-node policy only applies to future creates', async () => {
  const f = fixture(), running = await f.started('alice', 1), settings = f.hub.snapshot().nodes[0].settings
  f.hub.applySettings('test-node', { ...settings, draining: true, memoryMb: 256, ttlSeconds: 3600, idleSeconds: 600, lockVersion: 1 })
  assert.equal((await f.controller.start('alice', 1)).id, running.id)
  assert.equal(f.controller.status('alice').instance.policy.resources.memory_max_mb, 2048)
  await assert.rejects(f.controller.start('bob', 2), { statusCode: 503 })
  f.hub.applySettings('test-node', { ...settings, draining: false, lockVersion: 0 })
  assert.equal(f.hub.snapshot().nodes[0].settings.draining, true, 'stale save cannot undo draining')
  f.hub.applySettings('test-node', { ...settings, draining: false, memoryMb: 256, ttlSeconds: 3600, idleSeconds: 600, lockVersion: 2 })
  const task = f.controller.start('bob', 2), request = f.requests('create_box').at(-1)
  assert.equal(request.payload.policy.resources.memory_max_mb, 256); assert.equal(request.payload.ttl_seconds, 3600); assert.equal(request.payload.idle_seconds, 600)
  f.respond(request, { inventory_revision: 2, expires_at: 1700003600, sandbox_ip: '10.200.0.3' }); await task
})
test('lowering concurrency prevents new allocations even when existing slots are above the new range', async () => {
  const f = fixture(), settings = f.hub.snapshot().nodes[0].settings
  f.hub.applySettings('test-node', { ...settings, maxSlots: 3, lockVersion: 1 })
  const alice = await f.started('alice', 1), bob = await f.started('bob', 2), carol = await f.started('carol', 3)
  assert.equal(carol.slot, 3)
  for (const [user, record] of [['alice', alice], ['bob', bob]]) { const stop = f.controller.stopOwned(user, record.id); f.respond(f.requests('destroy_box').at(-1)); await stop }
  f.hub.applySettings('test-node', { ...settings, maxSlots: 1, lockVersion: 2 })
  await assert.rejects(f.controller.start('eve', 4), { statusCode: 503 })
})
