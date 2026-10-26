import test from 'node:test'
import assert from 'node:assert/strict'
import { createPrivateKey, createPublicKey } from 'node:crypto'
import { sandboxConfig, authorizeWorker } from '../server/sandbox/config.ts'
import { SandboxHub } from '../server/sandbox/hub.ts'
const environment = () => ({ SANDBOX_CONTROL_ENABLED: 'true', SANDBOX_CLUSTER_SECRET: 'test-cluster-identity-0000000000000000', SANDBOX_SIGNING_SEED_HEX: '11'.repeat(32), SANDBOX_NODES_JSON: JSON.stringify([{ id: 'test-node', label: '测试节点', publicDataUrl: 'ws://127.0.0.1:18443' }]) })
function fixture() {
  const config = sandboxConfig(environment()); let time = 1700000000000
  const hub = new SandboxHub(config, () => time)
  const peer = id => ({ id, sent: [], closed: [], send(raw) { this.sent.push(JSON.parse(raw)) }, close(...args) { this.closed.push(args) } })
  const a = peer('a'), b = peer('b')
  const send = (peer, type, payload) => hub.message(peer, JSON.stringify({ id: 'message-1', type, payload }))
  const register = (peer = a, boot = 'boot-1', extra = {}) => send(peer, 'register', { node_id: 'test-node', boot_id: boot, version: '2.0', public_data_url: 'ws://127.0.0.1:18443', data_port: 18443, capabilities: { cpu_cores: 4, total_mem_mb: 8192, os_version: 'linux/amd64', has_kvm: false }, ...extra })
  const heartbeat = (peer = a, extra = {}) => send(peer, 'heartbeat', { node_id: 'test-node', boot_id: 'boot-1', seq: 1, inventory_revision: 0, timestamp: Math.floor(time / 1000), load_avg: 0.25, mem_available_mb: 4000, disk_free_gb: 80, active_sandboxes: 0, ...extra })
  return { config, hub, a, b, send, register, heartbeat, advance: ms => time += ms, node: () => hub.snapshot().nodes[0] }
}
test('Compose capability requires network acceptance and replies are correlated and allow-listed',async()=>{
 const f=fixture();f.hub.open(f.a,'test-node');f.register();f.heartbeat(f.a,{compose_labs:{available:true,network_verified:false,protocol_version:2}})
 assert.equal(f.hub.composeNodes.has('test-node'),false)
 const payload={target_id:'00000000-0000-4000-8000-000000000001',owner_id:'00000000-0000-4000-8000-000000000002'}
 await assert.rejects(f.hub.composeRpc('test-node','boot-1','compose_prepare_status',payload))
 f.heartbeat(f.a,{seq:2,compose_labs:{available:true,network_verified:true,protocol_version:1}});assert.equal(f.hub.composeNodes.has('test-node'),false)
 f.heartbeat(f.a,{seq:3,compose_labs:{available:true,network_verified:true,protocol_version:2}})
 const promise=f.hub.composeRpc('test-node','boot-1','compose_prepare_status',payload),sent=f.a.sent.at(-1)
 f.hub.message(f.a,JSON.stringify({id:sent.id,type:'compose_prepare_status_resp',payload:{success:true,boot_id:'boot-1',...payload,job_state:'ready',variables:'must-not-return',deployment_id:'dep_'+'a'.repeat(64),summary:{services:[{name:'web',image:'sha256:'+'a'.repeat(64),password:'must-not-return'}],networks:['default'],entry_service:'web',entry_network:'default',entry_port:8080,report:[],document:'must-not-return'}}}))
 assert.ok(!JSON.stringify(await promise).includes('must-not-return'))
 const rejected=f.hub.composeRpc('test-node','boot-1','compose_prepare_status',payload),next=f.a.sent.at(-1)
 f.hub.message(f.a,JSON.stringify({id:next.id,type:'compose_prepare_status_resp',payload:{success:true,boot_id:'boot-1',...payload,owner_id:'other'}}));await assert.rejects(rejected);assert.equal(f.hub.composeNodes.has('test-node'),false)
})
test('sandbox defaults off and invalid configuration fails closed without echoing values', () => {
  assert.equal(sandboxConfig({}).enabled, false)
  for (const patch of [{ SANDBOX_CLUSTER_SECRET: 'short' }, { SANDBOX_SIGNING_SEED_HEX: 'wrong' }, { SANDBOX_NODES_JSON: '[secret-value' }, { SANDBOX_NODES_JSON: JSON.stringify([{ id: 'n', label: 'Node', publicDataUrl: 'https://secret-value' }]) }]) {
    const config = sandboxConfig({ ...environment(), ...patch })
    assert.equal(config.enabled, false); assert.ok(config.configurationError)
    assert.ok(!JSON.stringify(config).includes('secret-value'))
  }
  const env = environment(); env.SANDBOX_NODES_JSON = JSON.stringify([{ id: 'n', label: 'Node', publicDataUrl: 'wss://worker.example' }])
  assert.equal(sandboxConfig(env).enabled, true)
})
test('worker authentication requires server secret and allowlisted header identity; browser origins are rejected', () => {
  const c = sandboxConfig(environment()), h = new Headers({ authorization: `Bearer ${c.secret}`, 'x-node-id': 'test-node' })
  assert.equal(authorizeWorker(c, h), null)
  assert.equal(authorizeWorker(sandboxConfig({}), h), 503)
  h.set('origin', 'https://example.test'); assert.equal(authorizeWorker(c, h), 403); h.delete('origin')
  h.set('x-node-id', 'other'); assert.equal(authorizeWorker(c, h), 403)
  h.set('authorization', 'Bearer wrong'); assert.equal(authorizeWorker(c, h), 401)
})
test('registration syncs the exact Ed25519 public key and heartbeat exposes safe cloned observations', () => {
  const f = fixture(); f.hub.open(f.a, 'test-node'); f.register()
  assert.equal(f.node().state, 'waiting')
  const privateKey = createPrivateKey({ key: Buffer.concat([Buffer.from('302e020100300506032b657004220420', 'hex'), Buffer.from('11'.repeat(32), 'hex')]), type: 'pkcs8', format: 'der' })
  assert.equal(f.a.sent[0].payload.public_key, createPublicKey(privateKey).export({ type: 'spki', format: 'der' }).subarray(-32).toString('hex'))
  f.heartbeat(); assert.equal(f.node().state, 'online'); assert.equal(f.node().telemetry.freeDiskGb, 80)
  const snapshot = f.hub.snapshot(); snapshot.nodes[0].label = 'changed'
  assert.equal(f.node().label, '测试节点')
  assert.ok(!JSON.stringify(snapshot).includes(f.config.secret)); assert.ok(!JSON.stringify(snapshot).includes(f.config.publicKey))
  assert.equal(snapshot.schedulingEnabled, false)
})
test('identity mismatch, bad endpoint, pre-registration heartbeat and malformed payload close the connection', () => {
  for (const action of [f => f.register(f.a, 'boot-1', { node_id: 'other' }), f => f.register(f.a, 'boot-1', { public_data_url: 'wss://other.example' }), f => f.heartbeat(), f => f.hub.message(f.a, 'null'), f => f.hub.message(f.a, 'x'.repeat(1024 * 1024 + 1))]) {
    const f = fixture(); f.hub.open(f.a, 'test-node'); action(f)
    assert.equal(f.a.closed[0][0], 1008); assert.equal(f.node().state, 'offline')
  }
})
test('invalid inventories and unsafe numeric clocks are rejected', () => {
  for (const patch of [{ seq: Number.MAX_SAFE_INTEGER + 1 }, { active_sandboxes: 1 }, { active_sandboxes: 2, sandboxes: [{ id: 'sbx_a', state: 'running', last_active_at: 1700000000 }, { id: 'sbx_a', state: 'running', last_active_at: 1700000000 }] }, { active_sandboxes: 1, sandboxes: [{ id: '../bad', state: 'running', last_active_at: 1700000000 }] }]) {
    const f = fixture(); f.hub.open(f.a, 'test-node'); f.register(); f.heartbeat(f.a, patch)
    assert.equal(f.a.closed[0][0], 1008)
  }
})
test('old sequence or inventory revisions cannot overwrite telemetry or keep a worker online', () => {
  const f = fixture(); f.hub.open(f.a, 'test-node'); f.register(); f.heartbeat(f.a, { seq: 5, inventory_revision: 8 })
  f.advance(14000); f.heartbeat(f.a, { seq: 4, inventory_revision: 9, disk_free_gb: 1 }); f.heartbeat(f.a, { seq: 6, inventory_revision: 7, disk_free_gb: 1 })
  assert.equal(f.node().telemetry.freeDiskGb, 80)
  f.advance(1001); assert.equal(f.node().state, 'offline'); assert.equal(f.node().telemetry.freeDiskGb, 80)
})
test('same-boot reconnect retains clocks, old close cannot disconnect replacement, reboot clears stale telemetry', () => {
  const f = fixture(); f.hub.open(f.a, 'test-node'); f.register(); f.heartbeat(f.a, { seq: 10, inventory_revision: 5 })
  f.hub.open(f.b, 'test-node'); f.hub.close(f.a); f.register(f.b)
  f.heartbeat(f.a, { seq: 20 }); f.heartbeat(f.b, { seq: 9, inventory_revision: 5 }); assert.equal(f.node().state, 'waiting')
  f.heartbeat(f.b, { seq: 11, inventory_revision: 5 }); assert.equal(f.node().state, 'online')
  f.hub.close(f.b); f.hub.open(f.a, 'test-node'); f.register(f.a, 'boot-2')
  assert.equal(f.node().telemetry, null); assert.equal(f.node().lastHeartbeatAt, null)
  f.heartbeat(f.a, { boot_id: 'boot-2' }); assert.equal(f.node().state, 'online')
})
test('registration and initial heartbeat deadlines are enforced; inventory does not enable scheduling', () => {
  const f = fixture(); f.hub.open(f.a, 'test-node'); f.advance(10000); f.hub.sweep(); assert.equal(f.node().state, 'offline')
  f.hub.open(f.b, 'test-node'); f.register(f.b)
  f.heartbeat(f.b, { active_sandboxes: 1, sandboxes: [{ id: 'sbx_test', state: 'running', last_active_at: 1700000000, gui_running: true }] })
  assert.equal(f.node().sandboxes[0].guiRunning, true); assert.equal(f.hub.snapshot().schedulingEnabled, false)
  f.hub.shutdown(); assert.equal(f.node().state, 'offline')
})

test('target RPC correlates independent target identity and rejects generation changes', async () => {
 const f=fixture();f.hub.open(f.a,'test-node');f.register();f.heartbeat(f.a,{docker_targets:{available:true,instances:[]}})
 assert.ok(f.hub.targetNodes.has('test-node'))
 const request=f.hub.targetRpc('test-node','boot-1','target_inspect',{target_id:'tgt_0123456789abcdef',owner_id:'user-1'})
 const sent=f.a.sent.at(-1)
 f.hub.message(f.a,JSON.stringify({id:sent.id,type:'target_inspect_resp',payload:{boot_id:'boot-1',target_id:'tgt_0123456789abcdef',success:true,instance:{target_id:'tgt_0123456789abcdef',owner_id:'user-1',state:'running',ready:false,readiness:'unverified'}}}))
 assert.equal((await request).instance.state,'running')
 const lost=f.hub.targetRpc('test-node','boot-1','target_stop',{target_id:'tgt_0123456789abcdef',owner_id:'user-1'})
 f.hub.close(f.a);await assert.rejects(lost,/connection lost/);assert.ok(!f.hub.targetNodes.has('test-node'))
 f.hub.shutdown()
})

test('learner experiments retain node affinity across scopes and reject split placement', async () => {
 const {experimentNode}=await import('../shared/target-runtime.ts')
 assert.equal(experimentNode(null,[]),null)
 assert.equal(experimentNode(null,['a','a']),'a')
 assert.equal(experimentNode('a',['a']),'a')
 assert.throws(()=>experimentNode(null,['a','b']),/Conflicting/)
 assert.throws(()=>experimentNode('b',['a']),/Conflicting/)
})
test('only a confirmed stopped summary releases a target lifecycle', async () => {
 const {targetReplyState}=await import('../shared/target-runtime.ts')
 assert.equal(targetReplyState('starting','inspect',{error:'target_not_found'}),'unconfirmed')
 assert.equal(targetReplyState('running','start',{error:'target_already_stopped'}),'unconfirmed')
 assert.equal(targetReplyState('running','stop',{}),'stopping')
 assert.equal(targetReplyState('stopping','inspect',{instance:{state:'stopped'}}),'stopped')
 assert.equal(targetReplyState('running','inspect',{instance:{state:'missing'}}),'missing')
})
