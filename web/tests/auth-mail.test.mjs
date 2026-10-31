import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { spawn } from 'node:child_process'
import { once } from 'node:events'
import crypto from 'node:crypto'
import postgres from 'postgres'
import { chromium } from 'playwright'

test(
  'Resend account lifecycle without public tokens or real email delivery',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 90000 },
  async () => {
    const database = process.env.CONTENT_TEST_DATABASE_URL,
      url = new URL(database)
    assert.ok(['127.0.0.1', 'localhost'].includes(url.hostname) && url.pathname.endsWith('_test'))
    const sql = postgres(database, { max: 2 }),
      emails = [],
      registered = []
    let reject = false,
      app,
      browser,
      logs = ''
    const capture = createServer(async (req, res) => {
      let text = ''
      for await (const chunk of req) text += chunk
      assert.equal(req.headers.authorization, 'Bearer test-resend-only')
      if (reject) {
        res.writeHead(422)
        res.end(JSON.stringify({ message: 'PRIVATE provider failure' }))
        return
      }
      emails.push(JSON.parse(text))
      res.setHeader('content-type', 'application/json')
      res.end(JSON.stringify({ id: crypto.randomUUID() }))
    })
    capture.listen(0, '127.0.0.1')
    await once(capture, 'listening')
    const reservation = createServer()
    reservation.listen(0, '127.0.0.1')
    await once(reservation, 'listening')
    const port = reservation.address().port
    await new Promise(r => reservation.close(r))
    const base = `http://127.0.0.1:${port}`
    const req = async (path, body, cookie) => {
      const r = await fetch(base + path, {
        method: body ? 'POST' : 'GET',
        headers: {
          ...(body ? { 'content-type': 'application/json' } : {}),
          ...(cookie ? { cookie } : {})
        },
        body: body ? JSON.stringify(body) : undefined
      })
      return {
        status: r.status,
        json: await r.json(),
        cookie: r.headers.get('set-cookie')?.split(';')[0]
      }
    }
    const token = mail => new URL(mail.text.match(/http:\/\/\S+/)[0]).searchParams.get('token')
    const email = `mail-${crypto.randomUUID()}@example.test`,
      password = 'LocalTestPassword123!'
    registered.push(email)
    try {
      app = spawn(
        process.execPath,
        [
          '--import',
          new URL('./helpers/resend-capture.mjs', import.meta.url).pathname,
          new URL('../.output/server/index.mjs', import.meta.url).pathname
        ],
        {
          env: {
            ...process.env,
            NODE_ENV: 'test',
            AUTH_REGISTRATION_EMAIL_DOMAINS: 'example.test',
            DATABASE_URL: database,
            PORT: String(port),
            NITRO_PORT: String(port),
            NITRO_HOST: '127.0.0.1',
            HOST: '127.0.0.1',
            SANDBOX_CONTROL_ENABLED: 'false',
            RESEND_API_KEY: 'test-resend-only',
            MAIL_FROM: 'AttackBox <mail@example.test>',
            APP_BASE_URL: base,
            RESEND_CAPTURE_URL: `http://127.0.0.1:${capture.address().port}`
          },
          stdio: ['ignore', 'pipe', 'pipe']
        }
      )
      app.stdout.on('data', x => (logs += x))
      app.stderr.on('data', x => (logs += x))
      let ready = false
      for (let i = 0; i < 200; i++) {
        try {
          await fetch(base + '/api/auth/me')
          ready = true
          break
        } catch {
          await new Promise(r => setTimeout(r, 100))
        }
      }
      assert.ok(ready, 'local app started')
      const registeredResponse = await req('/api/auth/register', {
        email,
        password: 'UntrustedBeforeVerification123!'
      })
      assert.equal(registeredResponse.status, 200)
      assert.deepEqual(Object.keys(registeredResponse.json), ['message'])
      assert.equal(emails.length, 1)
      const verify = token(emails[0])
      assert.ok(verify)
      assert.equal(
        (await sql`select id from users where email=${email}`).length,
        0,
        'mail requests do not reserve accounts'
      )
      assert.equal((await req('/api/auth/login', { email, password })).status, 401)
      assert.deepEqual((await req('/api/auth/registration-token', { token: verify })).json, {
        email
      })
      await Promise.all(
        [1, 2, 3].map(() =>
          req('/api/auth/register', { email, password: 'IgnoredRetryPassword123!' })
        )
      )
      assert.equal(emails.length, 1, 'cooldown survives concurrent requests')
      assert.equal((await req('/api/auth/verify-email', { token: verify, password })).status, 200)
      assert.ok(
        [400, 403].includes(
          (await req('/api/auth/verify-email', { token: verify, password })).status
        )
      )
      assert.equal(
        (await req('/api/auth/login', { email, password: 'UntrustedBeforeVerification123!' }))
          .status,
        401
      )
      const beforeRepeat = emails.length
      assert.deepEqual((await req('/api/auth/register', { email })).json, registeredResponse.json)
      assert.equal(
        emails.length,
        beforeRepeat,
        'existing active accounts are neither modified nor emailed by registration'
      )
      const login = await req('/api/auth/login', { email, password })
      assert.equal(login.status, 200)
      const reset = await req('/api/auth/forgot-password', { email }),
        unknown = await req('/api/auth/forgot-password', {
          email: 'missing@example.test'
        })
      assert.deepEqual(reset.json, unknown.json)
      assert.equal(emails.length, 2)
      const resetToken = token(emails[1]),
        newPassword = 'ReplacementPassword123!'
      const concurrent = await Promise.all(
        [1, 2].map(() => req('/api/auth/reset-password', { token: resetToken, newPassword }))
      )
      assert.deepEqual(concurrent.map(x => x.status).sort(), [200, 400])
      assert.equal((await req('/api/auth/me', undefined, login.cookie)).status, 401)
      assert.equal((await req('/api/auth/login', { email, password })).status, 401)
      assert.equal((await req('/api/auth/login', { email, password: newPassword })).status, 200)
      await sql`update email_tokens set created_at=now()-interval '2 minutes' where target_email=${email}`
      await req('/api/auth/forgot-password', { email })
      const expired = token(emails.at(-1))
      await sql`update email_tokens set created_at=now()-interval '2 minutes',expires_at=now()-interval '1 minute' where target_email=${email}`
      assert.equal(
        (await req('/api/auth/reset-password', { token: expired, newPassword })).status,
        400
      )
      const beforeFailure = (
        await sql`select count(*)::int as n from email_tokens where target_email=${email}`
      )[0].n
      reject = true
      assert.deepEqual((await req('/api/auth/forgot-password', { email })).json, unknown.json)
      reject = false
      assert.equal(
        (await sql`select count(*)::int as n from email_tokens where target_email=${email}`)[0].n,
        beforeFailure
      )
      const [account] = await sql`select id from users where email=${email}`
      for (let i = 0; i < 3; i++)
        await sql`insert into email_tokens(user_id,target_email,purpose,token_hash,expires_at) values(${account.id},${email},'reset_password',${crypto.randomBytes(32)},now()+interval '30 minutes')`
      const sentCount = emails.length
      await req('/api/auth/forgot-password', { email })
      assert.equal(emails.length, sentCount, 'hourly recipient budget enforced')
      const pending = `pending-${crypto.randomUUID()}@example.test`
      registered.push(pending)
      reject = true
      assert.equal((await req('/api/auth/register', { email: pending, password })).status, 503)
      assert.equal(
        (await sql`select count(*)::int as n from email_tokens where target_email=${pending}`)[0].n,
        0,
        'failed send rolls back credentials'
      )
      assert.equal(
        (await sql`select id from users where email=${pending}`).length,
        0,
        'failed delivery must not create an account'
      )
      reject = false
      browser = await chromium.launch({
        executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const page = await browser.newPage()
      await page.goto(base + '/auth?mode=register&email=' + encodeURIComponent(pending))
      await page.getByRole('heading', { name: '从这里，开始。', exact: true }).waitFor()
      assert.equal(
        await page.locator('#email:enabled').inputValue(),
        pending,
        'registration preserves the requested mailbox'
      )
      await page.getByRole('button', { name: '发送验证邮件', exact: true }).click()
      await page.getByRole('heading', { name: '查收验证邮件。', exact: true }).waitFor()
      assert.equal(emails.at(-1).to[0], pending)
      assert.equal((await sql`select id from users where email=${pending}`).length, 0)
      const pendingToken = token(emails.at(-1))
      const claimed = await Promise.all(
        [1, 2].map(() => req('/api/auth/verify-email', { token: pendingToken, password }))
      )
      assert.deepEqual(
        claimed.map(r => r.status).sort(),
        [200, 400],
        'proof is consumed once under concurrent completion'
      )
      const browserEmail = `browser-${crypto.randomUUID()}@example.test`
      registered.push(browserEmail)
      await page.goto(base + '/auth?mode=register')
      await page.locator('#email:enabled').fill('blocked@mailinator.com')
      assert.equal(await page.locator('#password').count(), 0)
      await page.getByRole('button', { name: '发送验证邮件', exact: true }).click()
      await page.getByText('该邮箱域名暂不支持注册。', { exact: true }).waitFor()
      assert.equal(
        (await req('/api/auth/register', { email: 'blocked@mailinator.com' })).status,
        403
      )
      await page.locator('#email:enabled').fill(browserEmail)
      await page.getByRole('button', { name: '发送验证邮件', exact: true }).click()
      await page.getByRole('heading', { name: '查收验证邮件。', exact: true }).waitFor()
      assert.equal(emails.at(-1).to[0], browserEmail)
      assert.equal((await sql`select id from users where email=${browserEmail}`).length, 0)
      const browserToken = token(emails.at(-1))
      await page.goto(base + '/auth?mode=verify&token=' + browserToken)
      await page.locator('#password:enabled').fill(password)
      assert.equal(
        (await sql`select id from users where email=${browserEmail}`).length,
        0,
        'opening a link never consumes proof or creates an account'
      )
      await page.getByRole('button', { name: '完成注册', exact: true }).click()
      await page.getByRole('heading', { name: '账号创建成功。', exact: true }).waitFor()
      assert.equal(
        (await sql`select status from users where email=${browserEmail}`)[0].status,
        'active'
      )
      await page.getByRole('button', { name: '前往登录', exact: true }).click()
      assert.equal(await page.locator('#email').inputValue(), browserEmail)
      const oldPending = `unverified-${crypto.randomUUID()}@example.test`
      registered.push(oldPending)
      await sql`insert into users(email,display_name,password_hash,status) select ${oldPending},'Unverified',password_hash,'pending' from users where email=${email}`
      await req('/api/auth/register', { email: oldPending, password: 'IgnoredApplicantPassword!' })
      const ownerPassword = 'MailboxOwnerPassword123!'
      assert.equal(
        (
          await req('/api/auth/verify-email', {
            token: token(emails.at(-1)),
            password: ownerPassword
          })
        ).status,
        200
      )
      assert.equal(
        (await req('/api/auth/login', { email: oldPending, password: newPassword })).status,
        401,
        'claiming a pending account replaces previously proposed credentials'
      )
      assert.equal(
        (await req('/api/auth/login', { email: oldPending, password: ownerPassword })).status,
        200
      )
      const outside = `existing-${crypto.randomUUID()}@outside.test`
      registered.push(outside)
      await sql`insert into users(email,display_name,password_hash,status,email_verified_at) select ${outside},'Existing User',password_hash,'active',now() from users where email=${email}`
      assert.equal((await req('/api/auth/forgot-password', { email: outside })).status, 200)
      assert.equal(
        emails.at(-1).to[0],
        outside,
        'existing accounts outside the registration list can still recover passwords'
      )
      assert.ok(!logs.includes('test-resend-only'))
      assert.ok(!logs.includes(verify))
      assert.ok(!logs.includes(resetToken))
      assert.ok(!logs.includes('PRIVATE provider failure'))
      assert.match(logs, /reason: 'provider_rejected'/)
      assert.match(logs, /providerStatus: 422/)
      assert.ok(!logs.includes(pending), 'mail failure diagnostics must not contain the recipient')
    } finally {
      await browser?.close()
      if (app) {
        const done = once(app, 'exit')
        app.kill('SIGTERM')
        await done
      }
      await new Promise(r => capture.close(r))
      for (const email of registered) {
        await sql`delete from email_tokens where target_email=${email}`
        await sql`delete from users where email=${email}`
      }
      await sql.end()
    }
  }
)
