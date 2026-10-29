import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { once } from 'node:events'
import { spawn } from 'node:child_process'
import { randomUUID, createHash } from 'node:crypto'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'

test(
  'real login/register endpoints enforce Turnstile and atomic limits; widget gates submission and resets after failure',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 90000 },
  async () => {
    const originalBase = process.env.CONTENT_TEST_BASE_URL
    const seen = new Set(),
      calls = []
    let app,
      fixture,
      browser,
      logs = ''
    const capture = createServer(async (req, res) => {
      let text = ''
      for await (const part of req) text += part
      const body = new URLSearchParams(text),
        token = body.get('response') || ''
      calls.push(body)
      res.setHeader('content-type', 'application/json')
      if (token === 'outage') {
        res.writeHead(503)
        res.end('{}')
        return
      }
      const success = token.startsWith('valid-') && !seen.has(token)
      seen.add(token)
      res.end(
        JSON.stringify({
          success,
          hostname: token.includes('wrong-host') ? 'other.test' : '127.0.0.1',
          action: token.includes('register') ? 'register' : 'login'
        })
      )
    })
    capture.listen(0, '127.0.0.1')
    await once(capture, 'listening')
    const reservation = createServer()
    reservation.listen(0, '127.0.0.1')
    await once(reservation, 'listening')
    const port = reservation.address().port
    await new Promise(resolve => reservation.close(resolve))
    const base = `http://127.0.0.1:${port}`
    const request = async (path, body) => {
      const response = await fetch(base + path, {
        method: body ? 'POST' : 'GET',
        headers: body ? { 'content-type': 'application/json' } : {},
        body: body ? JSON.stringify(body) : undefined
      })
      return { response, json: await response.json() }
    }
    const token = () => 'valid-' + randomUUID()
    try {
      process.env.CONTENT_TEST_BASE_URL = base
      fixture = await learningFixture()
      const learner = await fixture.account()
      await fixture.sql`delete from auth_rate_limits`
      app = spawn(
        process.execPath,
        [
          '--import',
          new URL('./helpers/turnstile-capture.mjs', import.meta.url).pathname,
          new URL('../.output/server/index.mjs', import.meta.url).pathname
        ],
        {
          env: {
            ...process.env,
            NODE_ENV: 'test',
            DATABASE_URL: process.env.CONTENT_TEST_DATABASE_URL,
            HOST: '127.0.0.1',
            NITRO_HOST: '127.0.0.1',
            PORT: String(port),
            NITRO_PORT: String(port),
            SANDBOX_CONTROL_ENABLED: 'false',
            APP_BASE_URL: base,
            RESEND_API_KEY: 'not-used-test-key',
            MAIL_FROM: 'AttackBox <mail@example.test>',
            TURNSTILE_SITE_KEY: 'test-public-key',
            TURNSTILE_SECRET_KEY: 'test-private-secret',
            TURNSTILE_CAPTURE_URL: `http://127.0.0.1:${capture.address().port}`,
            AUTH_REGISTRATION_EMAIL_DOMAINS: 'example.test'
          },
          stdio: ['ignore', 'pipe', 'pipe']
        }
      )
      app.stdout.on('data', value => {
        logs += value
      })
      app.stderr.on('data', value => {
        logs += value
      })
      let ready = false
      for (let i = 0; i < 100; i++) {
        try {
          await fetch(base + '/api/auth/challenge')
          ready = true
          break
        } catch {
          await new Promise(resolve => setTimeout(resolve, 100))
        }
      }
      assert.ok(ready, logs)
      const publicConfig = await request('/api/auth/challenge')
      assert.deepEqual(publicConfig.json, { enabled: true, siteKey: 'test-public-key' })
      assert.equal(publicConfig.response.headers.get('cache-control'), 'private, no-store')
      const credentials = { email: learner.email, password: learner.password }
      const sessions = Number(
        (await fixture.sql`select count(*) from auth_sessions where user_id=${learner.id}`)[0].count
      )
      for (const turnstileToken of [
        undefined,
        true,
        'invalid',
        'valid-wrong-host',
        'valid-register-wrong-action'
      ]) {
        const result = await request('/api/auth/login', { ...credentials, turnstileToken })
        assert.equal(result.response.status, 403)
        assert.equal(result.response.headers.get('set-cookie'), null)
      }
      assert.equal(
        Number(
          (await fixture.sql`select count(*) from auth_sessions where user_id=${learner.id}`)[0]
            .count
        ),
        sessions
      )
      assert.equal(
        (await request('/api/auth/login', { ...credentials, turnstileToken: 'outage' })).response
          .status,
        503
      )
      assert.equal(
        (
          await request('/api/auth/register', {
            email: 'captcha@example.test',
            password: 'password-for-test'
          })
        ).response.status,
        403
      )
      assert.equal(
        (await fixture.sql`select id from users where email='captcha@example.test'`).length,
        0
      )
      assert.equal(
        (
          await request('/api/auth/register', {
            ...credentials,
            turnstileToken: 'valid-register-existing-user'
          })
        ).response.status,
        409
      )
      const onceToken = token()
      const success = await request('/api/auth/login', {
        ...credentials,
        turnstileToken: onceToken
      })
      assert.equal(success.response.status, 200)
      assert.ok(success.response.headers.get('set-cookie')?.includes('HttpOnly'))
      assert.equal(success.json.user.id, learner.id)
      assert.equal(
        (await request('/api/auth/login', { ...credentials, turnstileToken: onceToken })).response
          .status,
        403
      )
      assert.equal(calls[0].get('secret'), 'test-private-secret')
      assert.equal(calls[0].get('remoteip'), '127.0.0.1')

      // Unknown users avoid password hashing cost but consume the identical pair limit.
      await fixture.sql`delete from auth_rate_limits`
      const unknown = 'missing-' + randomUUID() + '@example.test'
      const attempts = await Promise.all(
        Array.from({ length: 12 }, () =>
          request('/api/auth/login', {
            email: unknown,
            password: 'wrong-password',
            turnstileToken: token()
          })
        )
      )
      assert.equal(attempts.filter(result => result.response.status === 401).length, 8)
      assert.equal(attempts.filter(result => result.response.status === 429).length, 4)
      assert.ok(
        Number(
          attempts
            .find(result => result.response.status === 429)
            .response.headers.get('retry-after')
        ) > 0
      )
      const learnerPair = createHash('sha256')
        .update(JSON.stringify(['login-pair', '127.0.0.1', learner.email]))
        .digest('hex')
      assert.equal(
        (
          await request('/api/auth/login', {
            ...credentials,
            password: 'wrong-password',
            turnstileToken: token()
          })
        ).response.status,
        401
      )
      assert.equal(
        (await fixture.sql`select attempts from auth_rate_limits where key=${learnerPair}`)[0]
          .attempts,
        1
      )
      assert.equal(
        (await request('/api/auth/login', { ...credentials, turnstileToken: token() })).response
          .status,
        200
      )
      assert.equal(
        (await fixture.sql`select key from auth_rate_limits where key=${learnerPair}`).length,
        0
      )
      const pairKey = createHash('sha256')
        .update(JSON.stringify(['login-pair', '127.0.0.1', unknown]))
        .digest('hex')
      await fixture.sql`update auth_rate_limits set window_at=now()-interval '16 minutes' where key=${pairKey}`
      assert.equal(
        (
          await request('/api/auth/login', {
            email: unknown,
            password: 'wrong-password',
            turnstileToken: token()
          })
        ).response.status,
        401
      )
      const ipKey = createHash('sha256')
        .update(JSON.stringify(['auth-ip', '127.0.0.1']))
        .digest('hex')
      await fixture.sql`update auth_rate_limits set attempts=60 where key=${ipKey}`
      const before = calls.length
      const throttled = await request('/api/auth/login', {
        ...credentials,
        turnstileToken: token()
      })
      assert.equal(throttled.response.status, 429)
      assert.equal(calls.length, before, 'IP throttling happens before external verification')
      await fixture.sql`delete from auth_rate_limits`

      browser = await chromium.launch({
        executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const page = await browser.newPage(),
        errors = []
      page.on('pageerror', error => errors.push(error.message))
      // Model the third-party widget callbacks; backend still validates through the capture server.
      await page.route('https://challenges.cloudflare.com/turnstile/v0/api.js*', route =>
        route.fulfill({
          contentType: 'application/javascript',
          body: `
      window.turnstile={render(el,opts){window.challenge=opts;window.resets=0;return 'test-widget'},reset(){window.resets++},remove(){}};
    `
        })
      )
      await page.goto(base + '/auth')
      await page.locator('#email:enabled').waitFor()
      await page.locator('#email').fill(learner.email)
      await page.locator('#password').fill('wrong-password')
      const submit = page.getByRole('button', { name: '登录', exact: true })
      await page.waitForFunction(() => !!window.challenge)
      assert.equal(await submit.isDisabled(), true)
      await page.evaluate(() => window.challenge.callback('valid-browser-bad-password'))
      await submit.click()
      await page.waitForFunction(() => window.resets === 1)
      assert.equal(await submit.isDisabled(), true)
      assert.equal(new URL(page.url()).pathname, '/auth')
      await page.evaluate(() => window.challenge.callback('valid-browser-expire'))
      await page.waitForFunction(() => !document.querySelector('button[type=submit]').disabled)
      await page.evaluate(() => window.challenge['expired-callback']())
      assert.equal(await submit.isDisabled(), true)
      await page.locator('#password').fill(learner.password)
      await page.evaluate(() => window.challenge.callback('valid-browser-login'))
      await submit.click()
      await page.waitForURL('**/learn')
      assert.deepEqual(errors, [])

      const retryPage = await browser.newPage()
      let sdkRequests = 0
      await retryPage.route('https://challenges.cloudflare.com/turnstile/v0/api.js*', route => {
        sdkRequests++
        if (sdkRequests === 1) return route.abort()
        return route.fulfill({
          contentType: 'application/javascript',
          body: `
        window.turnstile={render(el,opts){window.challenge=opts;return 'retry-widget'},reset(){},remove(){}};
      `
        })
      })
      await retryPage.goto(base + '/auth?mode=register')
      await retryPage.getByRole('alert').filter({ hasText: '人机验证加载失败' }).waitFor()
      assert.equal(await retryPage.locator('button[type=submit]').isDisabled(), true)
      await retryPage.getByRole('button', { name: '重试验证' }).click()
      await retryPage.waitForFunction(() => !!window.challenge)
      assert.equal(await retryPage.evaluate(() => window.challenge.action), 'register')
      await retryPage.evaluate(() => window.challenge.callback('valid-register-browser'))
      await retryPage.waitForFunction(() => !document.querySelector('button[type=submit]').disabled)
      assert.equal(sdkRequests, 2)
      assert.equal(
        await retryPage.locator('[modelvalue]').count(),
        0,
        'challenge tokens do not become DOM attributes'
      )
      assert.ok(!logs.includes('test-private-secret'))
    } finally {
      if (browser) await browser.close()
      if (app) {
        const closed = once(app, 'exit')
        app.kill()
        await closed
      }
      await new Promise(resolve => capture.close(resolve))
      if (fixture) {
        await fixture.sql`delete from auth_rate_limits`
        await fixture.cleanup()
      }
      if (originalBase === undefined) delete process.env.CONTENT_TEST_BASE_URL
      else process.env.CONTENT_TEST_BASE_URL = originalBase
    }
  }
)
