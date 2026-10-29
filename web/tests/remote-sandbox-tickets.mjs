import assert from 'node:assert/strict'
import { readFileSync, writeFileSync } from 'node:fs'
import { once } from 'node:events'
import WebSocket from 'ws'
if (process.env.RUN_REAL_SANDBOX_ACCEPTANCE !== 'true' || !process.env.SANDBOX_ACCEPTANCE_FIXTURE)
  throw Error('Explicit acceptance opt-in required')
const actors = JSON.parse(readFileSync(process.env.SANDBOX_ACCEPTANCE_FIXTURE)),
  base = 'http://127.0.0.1:3031',
  saved = '/tmp/attackbox-real-expired-ticket.json'
async function req(path, body) {
  const r = await fetch(base + path, {
    method: body ? 'POST' : 'GET',
    headers: {
      cookie: `attackbox_session=${actors.alice.token}`,
      ...(body ? { 'content-type': 'application/json' } : {})
    },
    body: body ? JSON.stringify(body) : undefined
  })
  assert.equal(r.status, 200)
  return (await r.json()).data
}
function ticketURL(t) {
  const u = new URL(t.url)
  assert.equal(u.origin, 'ws://127.0.0.1:18443')
  u.searchParams.set('ticket', t.ticket)
  return u.toString()
}
function rejected(url, origin = base) {
  return new Promise((resolve, reject) => {
    let received = false
    const s = new WebSocket(url, { origin })
    s.on('unexpected-response', (_, r) => {
      received = true
      r.resume()
      s.terminate()
      resolve(r.statusCode)
    })
    s.on('open', () => {
      s.close()
      reject(Error('Invalid ticket unexpectedly accepted'))
    })
    s.on('error', e => {
      if (!received) reject(e)
    })
  })
}
if (process.argv[2] === 'expired') {
  const record = JSON.parse(readFileSync(saved))
  assert.ok(Date.now() > Date.parse(record.expiresAt))
  assert.equal(await rejected(ticketURL(record)), 403)
  console.log('PASS unused expired real Worker ticket rejected')
} else {
  let instance
  const deadline = Date.now() + 100000
  while (Date.now() < deadline) {
    instance = (await req('/api/sandbox/status')).instance
    if (
      instance?.state === 'running' &&
      Date.parse(instance.expiresAt) - Date.parse(instance.startedAt) <= 65000
    )
      break
    await new Promise(r => setTimeout(r, 500))
  }
  assert.ok(
    instance?.state === 'running' &&
      Date.parse(instance.expiresAt) - Date.parse(instance.startedAt) <= 65000,
    '60-second TTL acceptance instance required'
  )
  const unused = await req('/api/sandbox/ticket', { sandboxId: instance.id })
  writeFileSync(saved, JSON.stringify(unused), { mode: 0o600 })
  const single = await req('/api/sandbox/ticket', { sandboxId: instance.id }),
    url = ticketURL(single)
  const connection = new WebSocket(url, { origin: base })
  await once(connection, 'open')
  connection.close()
  assert.equal(await rejected(url), 403)
  const bad = await req('/api/sandbox/ticket', { sandboxId: instance.id })
  bad.ticket = bad.ticket.slice(0, -4) + 'xxxx'
  assert.equal(await rejected(ticketURL(bad)), 403)
  const origin = await req('/api/sandbox/ticket', { sandboxId: instance.id })
  assert.equal(await rejected(ticketURL(origin), 'https://other.invalid'), 403)
  console.log(
    'PASS real Worker single-use replay, signature tampering and foreign Origin rejected; unused expiry ticket saved'
  )
}
