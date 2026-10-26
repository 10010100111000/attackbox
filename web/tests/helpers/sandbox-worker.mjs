// Local protocol fixture only. It simulates command output; it is not a sandbox.
import http from 'node:http'
import { verify, createHash, createPublicKey } from 'node:crypto'
import { once } from 'node:events'
import WebSocket, { WebSocketServer } from 'ws'
import { testDesktop } from './rfb-desktop.mjs'
export async function sandboxWorker(base, secret, options = {}) {
  if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname)) throw new Error('Local test service required')
  const requests = [], tickets = [], terminals = [], desktops = [], desktopInputs = [], commands = [], boxes = new Map(), consumed = new Set()
  let key, publicKey, seq = 0, revision = 0
  const boot = 'fixture-' + Math.random().toString(16).slice(2)
  const composeJobs=new Map(),composeInstances=new Map()
  const httpServer = http.createServer((_, res) => { res.writeHead(404); res.end() })
  const gateway = new WebSocketServer({ noServer: true })
  httpServer.on('upgrade', (req, socket, head) => {
    try {
      const url = new URL(req.url, 'http://127.0.0.1'), token = url.searchParams.get('ticket') || '', [encoded, signature] = token.split('.')
      const payload = Buffer.from(encoded, 'base64url'), claims = JSON.parse(payload)
      if (!['/ws/cli', '/ws/gui'].includes(url.pathname) || !key || !verify(null, payload, key, Buffer.from(signature, 'base64url')) || claims.typ !== (url.pathname === '/ws/gui' ? 'vnc' : 'pty') || claims.exp < Date.now() / 1000 || consumed.has(claims.jti) || boxes.get(claims.sbx)?.user_id !== claims.sub) throw new Error('Invalid ticket')
      consumed.add(claims.jti); tickets.push(claims)
      gateway.handleUpgrade(req, socket, head, ws => { ws.box = claims.sbx; ws.stream = claims.typ; ws.windowId = url.searchParams.get('win'); gateway.emit('connection', ws) })
    } catch { socket.end('HTTP/1.1 403 Forbidden\r\nConnection: close\r\n\r\n') }
  })
  gateway.on('connection', socket => {
    if (socket.stream === 'vnc') { desktops.push(socket); boxes.get(socket.box).guiRunning = true; testDesktop(socket, desktopInputs); heartbeat(); return }
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
    send('heartbeat', { node_id: 'test-node', boot_id: boot, seq: ++seq, inventory_revision: revision, timestamp: Math.floor(Date.now() / 1000), load_avg: 0.3, mem_available_mb: 16384, disk_free_gb: 80, ...(options.composeLabs?{compose_labs:{available:true,network_verified:true,protocol_version:2}}:{}), active_sandboxes: boxes.size, sandboxes: [...boxes.keys()].map(id => ({ id, state: 'running', last_active_at: Math.floor(Date.now() / 1000), gui_running: !!boxes.get(id)?.guiRunning })) })
  }
  let readyResolve
  const ready = new Promise(resolve => readyResolve = resolve)
  control.on('message', async raw => {
    const msg = JSON.parse(raw)
    if (msg.type === 'register_ack') {
      publicKey = msg.payload.public_key
      key = createPublicKey({ key: Buffer.concat([Buffer.from('302a300506032b6570032100', 'hex'), Buffer.from(msg.payload.public_key, 'hex')]), type: 'spki', format: 'der' }); heartbeat()
    } else if (msg.type === 'heartbeat_ack') readyResolve()
    else if(options.composeLabs&&msg.type.startsWith('compose_')){
      requests.push(msg);const p=msg.payload;const response={success:true,boot_id:boot,target_id:p.target_id,owner_id:p.owner_id}
      if(msg.type==='compose_prepare'){
        const downloaded=await fetch(base+'/api/worker/lab-bundles/'+p.bundle_id,{headers:{Authorization:`Bearer ${secret}`,'X-Node-ID':'test-node'}})
        if(!downloaded.ok){send(msg.type+'_resp',{...response,success:false,error:'bundle_download_failed'},msg.id);return}
        const bundle=await downloaded.json();if(bundle.hash!==p.bundle_hash)throw Error('Fixture bundle hash mismatch')
        composeJobs.set(p.preparation_id,{job_state:'preparing',deployment_id:p.rebuild?'dep_'+createHash('sha256').update(p.preparation_id).digest('hex'):'dep_'+'a'.repeat(64),summary:{services:[{name:'web',image:'sha256:'+'a'.repeat(64)}],networks:['default'],entry_service:p.settings.entryService,entry_network:p.settings.entryNetwork,entry_port:p.settings.entryPort,report:[]}})
        send(msg.type+'_resp',{...response,job_state:'preparing',logs:['已接收准备任务']},msg.id)
      }else if(msg.type==='compose_prepare_status'){
       const job=composeJobs.get(p.preparation_id);if(job&&job.job_state==='preparing'&&!options.composePrepareHold)job.job_state=options.composePrepareFail?'failed':'ready'
       const result={...response,...(job?.job_state==='ready'?job:{}),job_state:job?.job_state||'unknown',...(job?.job_state==='failed'?{error:'compose_prepare_failed'}:{}),logs:['部署包校验完成','镜像准备完成']}
       const gate=options.composeStatusGate;if(gate){gate.enter();await gate.promise}
       send(msg.type+'_resp',result,msg.id)
      }else if(msg.type==='compose_prepare_cancel'){
       const job=composeJobs.get(p.preparation_id);if(!job||job.job_state!=='ready')composeJobs.set(p.preparation_id,{job_state:'cancelled'})
       send(msg.type+'_resp',{...response,...composeJobs.get(p.preparation_id),logs:['准备任务已取消']},msg.id)
      }
      else{
        let record=composeInstances.get(p.target_id)
        if(msg.type==='compose_start'&&!record){record={target_id:p.target_id,owner_id:p.owner_id,expires_at:p.expires_at,ready:true,readiness:'healthy',entry_address:'10.202.0.2:8080',services:[{name:'web',state:'running',health:'healthy',ips:{default:'10.202.0.2'}}],state:'running'};composeInstances.set(p.target_id,record)}
        if(msg.type==='compose_stop'){record={...record,target_id:p.target_id,owner_id:p.owner_id,ready:false,readiness:'not_running',state:'stopped',services:[]};composeInstances.set(p.target_id,record)}
        const attached=[...boxes.values()].some(b=>b.owner_id===p.owner_id)
        send(msg.type+'_resp',{...response,instance:{...record,attacker_address:attached?'10.202.0.14':undefined,interface:attached?'lab0':undefined,attachment:record?.state==='stopped'?'disconnected':attached?'connected':'not_connected'}},msg.id)
      }
    }
    else if (msg.type === 'create_box') {
      requests.push(msg); boxes.set(msg.payload.sandbox_id, msg.payload); revision++
      send('create_box_resp', { success: true, boot_id: boot, sandbox_id: msg.payload.sandbox_id, inventory_revision: revision, expires_at: Math.floor(Date.now() / 1000) + msg.payload.ttl_seconds, sandbox_ip: `10.200.0.${msg.payload.slot_index + 1}` }, msg.id); heartbeat()
    } else if (msg.type === 'destroy_box') {
      requests.push(msg)
      if (options.failDestroy?.()) {
        send('destroy_box_resp', { success: false, boot_id: boot, sandbox_id: msg.payload.sandbox_id, error: 'Fixture cleanup failure' }, msg.id); return
      }
      boxes.delete(msg.payload.sandbox_id); revision++
      for (const socket of [...terminals, ...desktops]) if (socket.box === msg.payload.sandbox_id) socket.close(1000, 'Sandbox released')
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
    for (const socket of [...terminals, ...desktops]) socket.terminate()
    gateway.close(); await new Promise(resolve => httpServer.close(resolve))
  }
  return { boot, requests, tickets, terminals, desktops, desktopInputs, commands, boxes, close, get publicKey() { return publicKey } }
}
