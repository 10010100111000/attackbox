// Local protocol fixture only. It simulates command output; it is not a sandbox.
import http from 'node:http'
import { verify, createPublicKey } from 'node:crypto'
import { once } from 'node:events'
import WebSocket, { WebSocketServer } from 'ws'
export async function sandboxWorker(base, secret) {
  if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname)) throw new Error('Local test service required')
  const requests = [], tickets = [], terminals = [], commands = [], boxes = new Map(), consumed = new Set()
  let key, publicKey, seq = 0, revision = 0
  const boot = 'fixture-' + Math.random().toString(16).slice(2)
  const httpServer = http.createServer((_, res) => { res.writeHead(404); res.end() })
  const gateway = new WebSocketServer({ noServer: true })
  httpServer.on('upgrade', (req, socket, head) => {
    try {
      const url = new URL(req.url, 'http://127.0.0.1'), token = url.searchParams.get('ticket') || '', [encoded, signature] = token.split('.')
      const payload = Buffer.from(encoded, 'base64url'), claims = JSON.parse(payload)
      if (url.pathname !== '/ws/cli' || !key || !verify(null, payload, key, Buffer.from(signature, 'base64url')) || claims.typ !== 'pty' || claims.exp < Date.now() / 1000 || consumed.has(claims.jti) || boxes.get(claims.sbx)?.user_id !== claims.sub) throw new Error('Invalid ticket')
      consumed.add(claims.jti); tickets.push(claims)
      gateway.handleUpgrade(req, socket, head, ws => { ws.box = claims.sbx; ws.windowId = url.searchParams.get('win'); gateway.emit('connection', ws) })
    } catch { socket.end('HTTP/1.1 403 Forbidden\r\nConnection: close\r\n\r\n') }
  })
  gateway.on('connection', socket => {
    terminals.push(socket); socket.send(Buffer.from('box@fixture:~$ ')); let buffer = ''
    socket.on('message', (data, binary) => {
      if (!binary) return // Worker uses text frames for resize.
      const value = data.toString(); buffer += value; socket.send(Buffer.from(value.replace(/\r/g, '\r\n')))
      if (buffer.includes('\r')) {
        const command = buffer.trim(); commands.push(command); buffer = ''
        socket.send(Buffer.from(`Executed: ${command}\r\nbox@fixture:~$ `))
      }
    })
  })
  await new Promise(resolve => httpServer.listen(18443, '127.0.0.1', resolve))
  const control = new WebSocket(base.replace('http:', 'ws:') + '/cluster/connect', { headers: { Authorization: `Bearer ${secret}`, 'X-Node-ID': 'test-node' } })
  function send(type, payload, id = 'fixture-' + seq) { control.send(JSON.stringify({ id, type, payload })) }
  function heartbeat() {
    if (control.readyState !== WebSocket.OPEN) return
    send('heartbeat', { node_id: 'test-node', boot_id: boot, seq: ++seq, inventory_revision: revision, timestamp: Math.floor(Date.now() / 1000), load_avg: 0.3, mem_available_mb: 16384, disk_free_gb: 80, active_sandboxes: boxes.size, sandboxes: [...boxes.keys()].map(id => ({ id, state: 'running', last_active_at: Math.floor(Date.now() / 1000) })) })
  }
  let readyResolve
  const ready = new Promise(resolve => readyResolve = resolve)
  control.on('message', raw => {
    const msg = JSON.parse(raw)
    if (msg.type === 'register_ack') {
      publicKey = msg.payload.public_key
      key = createPublicKey({ key: Buffer.concat([Buffer.from('302a300506032b6570032100', 'hex'), Buffer.from(msg.payload.public_key, 'hex')]), type: 'spki', format: 'der' }); heartbeat()
    } else if (msg.type === 'heartbeat_ack') readyResolve()
    else if (msg.type === 'create_box') {
      requests.push(msg); boxes.set(msg.payload.sandbox_id, msg.payload); revision++
      send('create_box_resp', { success: true, boot_id: boot, sandbox_id: msg.payload.sandbox_id, inventory_revision: revision, expires_at: Math.floor(Date.now() / 1000) + msg.payload.ttl_seconds, sandbox_ip: `10.200.0.${msg.payload.slot_index + 1}` }, msg.id); heartbeat()
    } else if (msg.type === 'destroy_box') {
      requests.push(msg); boxes.delete(msg.payload.sandbox_id); revision++
      for (const socket of terminals) if (socket.box === msg.payload.sandbox_id) socket.close(1000, 'Sandbox released')
      send('destroy_box_resp', { success: true, boot_id: boot, sandbox_id: msg.payload.sandbox_id }, msg.id); heartbeat()
    }
  })
  await once(control, 'open')
  send('register', { node_id: 'test-node', boot_id: boot, version: '2.0', data_port: 18443, public_data_url: 'ws://127.0.0.1:18443', capabilities: { cpu_cores: 8, total_mem_mb: 16384, os_version: 'linux/amd64', has_kvm: false } })
  const readyTimer = setTimeout(() => control.terminate(), 5000)
  await Promise.race([ready, once(control, 'close').then(() => { throw new Error('Worker registration failed') })]); clearTimeout(readyTimer)
  const timer = setInterval(heartbeat, 1000)
  async function close() {
    clearInterval(timer); control.terminate()
    for (const socket of terminals) socket.terminate()
    gateway.close(); await new Promise(resolve => httpServer.close(resolve))
  }
  return { requests, tickets, terminals, commands, boxes, close, get publicKey() { return publicKey } }
}
