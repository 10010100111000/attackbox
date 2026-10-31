import test from 'node:test'
import assert from 'node:assert/strict'
import { createJiti } from 'jiti'
const jiti = createJiti(import.meta.url)
const { turnstileConfig, verifyTurnstile } = await jiti.import('../server/utils/turnstile.ts')
const { clientIp, trustedProxy } = await jiti.import('../server/utils/client-ip.ts')
const config = turnstileConfig({
  TURNSTILE_SITE_KEY: 'public-site',
  TURNSTILE_SECRET_KEY: 'private-secret',
  APP_BASE_URL: 'https://microsb.de'
})
const status = code => error =>
  error.statusCode === code && !error.message.includes('private-secret')

test('Turnstile is optional only when both keys are absent; incomplete configuration fails closed', () => {
  assert.equal(turnstileConfig({}).enabled, false)
  assert.deepEqual(config, {
    enabled: true,
    siteKey: 'public-site',
    secretKey: 'private-secret',
    hostname: 'microsb.de'
  })
  for (const env of [
    { TURNSTILE_SITE_KEY: 'site' },
    { TURNSTILE_SECRET_KEY: 'secret' },
    {
      ...process.env,
      TURNSTILE_SITE_KEY: 'site',
      TURNSTILE_SECRET_KEY: 'secret',
      APP_BASE_URL: 'http://microsb.de'
    },
    {
      TURNSTILE_SITE_KEY: 'site',
      TURNSTILE_SECRET_KEY: 'secret',
      APP_BASE_URL: 'https://user:password@microsb.de'
    },
    {
      TURNSTILE_SITE_KEY: 'site',
      TURNSTILE_SECRET_KEY: 'secret',
      APP_BASE_URL: 'https://microsb.de/path'
    }
  ])
    assert.throws(() => turnstileConfig(env), status(503))
})

test('verification requires a server-validated token bound to hostname and action', async () => {
  const calls = []
  const provider = async (url, init) => {
    calls.push({ url, init })
    return Response.json({ success: true, hostname: 'microsb.de', action: 'login' })
  }
  for (const token of [undefined, true, '', 'x'.repeat(2049)]) {
    await assert.rejects(verifyTurnstile(token, 'login', undefined, config, provider), status(403))
  }
  assert.equal(calls.length, 0)
  await verifyTurnstile('single-use-token', 'login', '203.0.113.8', config, provider)
  assert.equal(calls[0].url, 'https://challenges.cloudflare.com/turnstile/v0/siteverify')
  assert.equal(calls[0].init.redirect, 'error')
  assert.equal(calls[0].init.body.get('secret'), 'private-secret')
  assert.equal(calls[0].init.body.get('remoteip'), '203.0.113.8')
  assert.equal(calls[0].init.body.get('response'), 'single-use-token')
  for (const payload of [
    null,
    {},
    { success: false },
    { success: true, hostname: 'evil.test', action: 'login' },
    { success: true, hostname: 'microsb.de', action: 'register' },
    { success: true, hostname: 123, action: 'login' }
  ]) {
    await assert.rejects(
      verifyTurnstile('token', 'login', undefined, config, async () => Response.json(payload)),
      status(403)
    )
  }
})

test('provider failures fail closed and tokens cannot be reused when provider rejects them', async () => {
  for (const provider of [
    async () => {
      throw Error('private-secret')
    },
    async () => new Response('', { status: 502 }),
    async () => new Response('not JSON')
  ]) {
    await assert.rejects(
      verifyTurnstile('token', 'login', undefined, config, provider),
      status(503)
    )
  }
  let used = false
  const provider = async () => {
    const response = Response.json({
      success: !used,
      hostname: 'microsb.de',
      action: 'register',
      'error-codes': used ? ['timeout-or-duplicate'] : []
    })
    used = true
    return response
  }
  await verifyTurnstile('once', 'register', undefined, config, provider)
  await assert.rejects(
    verifyTurnstile('once', 'register', undefined, config, provider),
    status(403)
  )
})

test('forwarded client addresses are trusted only from configured proxy peers', () => {
  const event = (peer, headers = {}) => ({
    node: { req: { socket: { remoteAddress: peer }, headers } }
  })
  assert.equal(trustedProxy('127.0.0.1'), true)
  assert.equal(trustedProxy('::ffff:127.0.0.1'), true)
  assert.equal(trustedProxy('203.0.113.8'), false)
  assert.equal(trustedProxy('10.1.0.5', '10.1.0.0/24'), true)
  for (const invalid of ['*', '127.0.0.1', '127.0.0.1/99', 'bad/24', '127.0.0.1/8/24']) {
    assert.throws(() => trustedProxy('127.0.0.1', invalid), status(503))
  }
  assert.equal(
    clientIp(
      event('203.0.113.8', {
        'x-real-ip': '1.2.3.4',
        'x-forwarded-for': '5.6.7.8',
        'cf-connecting-ip': '9.9.9.9'
      })
    ),
    '203.0.113.8'
  )
  assert.equal(clientIp(event('127.0.0.1', { 'x-real-ip': '203.0.113.8' })), '203.0.113.8')
  assert.equal(
    clientIp(event('::ffff:127.0.0.1', { 'x-forwarded-for': '2001:db8::1' })),
    '2001:db8::1'
  )
  assert.equal(clientIp(event('127.0.0.1', { 'x-forwarded-for': '1.2.3.4, 5.6.7.8' })), '5.6.7.8')
  assert.equal(clientIp(event('127.0.0.1', { 'cf-connecting-ip': '1.2.3.4' })), '127.0.0.1')
})

test('multi-hop forwarding stops at the trust boundary and rejects malformed chains', () => {
  const previous = process.env.AUTH_TRUSTED_PROXY_CIDRS
  const event = (forwarded, extra = {}, peer = '::ffff:127.0.0.1') => ({
    node: {
      req: { socket: { remoteAddress: peer }, headers: { 'x-forwarded-for': forwarded, ...extra } }
    }
  })
  try {
    process.env.AUTH_TRUSTED_PROXY_CIDRS = '127.0.0.0/8,::1/128,10.1.0.0/24'
    assert.equal(clientIp(event('203.0.113.8, 10.1.0.5')), '203.0.113.8')
    assert.equal(clientIp(event('1.2.3.4, 203.0.113.8, 10.1.0.5')), '203.0.113.8')
    assert.equal(clientIp(event(' 2001:db8::1 , ::FFFF:127.0.0.2 ')), '2001:db8::1')
    assert.equal(clientIp(event('::ffff:203.0.113.8, 10.1.0.5')), '203.0.113.8')
    assert.equal(clientIp(event('1.2.3.4', {}, '203.0.113.8')), '203.0.113.8')
    assert.equal(
      clientIp(event('1.2.3.4, 10.1.0.5', { 'x-real-ip': '203.0.113.9' })),
      '203.0.113.9'
    )
    assert.equal(
      clientIp(event('203.0.113.8, 10.1.0.5', { 'x-real-ip': 'invalid' })),
      '203.0.113.8'
    )
    for (const chain of [
      '',
      'invalid',
      '1.2.3.4,',
      ', 10.1.0.5',
      '1.2.3.4:80',
      'invalid, 10.1.0.5'
    ]) {
      assert.equal(clientIp(event(chain)), '127.0.0.1')
    }
    // A changed configuration must invalidate the parsed cache, including failures.
    process.env.AUTH_TRUSTED_PROXY_CIDRS = 'invalid'
    for (let i = 0; i < 2; i++) assert.throws(() => clientIp(event('203.0.113.8')), status(503))
    process.env.AUTH_TRUSTED_PROXY_CIDRS = '127.0.0.0/8'
    assert.equal(clientIp(event('203.0.113.8, 10.1.0.5')), '10.1.0.5')
  } finally {
    if (previous === undefined) delete process.env.AUTH_TRUSTED_PROXY_CIDRS
    else process.env.AUTH_TRUSTED_PROXY_CIDRS = previous
  }
})
