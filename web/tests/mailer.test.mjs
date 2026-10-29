import test from 'node:test'
import assert from 'node:assert/strict'
import { createResendMailer, mailConfig } from '../server/utils/mailer.ts'
const config = {
  apiKey: 'fake-test-key',
  from: 'AttackBox <hello@example.test>',
  baseUrl: 'https://learn.example.test'
}
test('Resend sends absolute verification/reset links with private credentials and idempotency', async () => {
  const calls = []
  const mail = createResendMailer(config, async (url, options) => {
    calls.push({ url, options })
    return new Response(JSON.stringify({ id: 'fake-mail-id' }))
  })
  await mail.sendVerificationEmail('user@example.test', 'verify-test-token')
  await mail.sendPasswordResetEmail('user@example.test', 'reset-test-token')
  assert.equal(calls.length, 2)
  for (const c of calls) {
    assert.equal(c.url, 'https://api.resend.com/emails')
    assert.equal(c.options.headers.Authorization, 'Bearer fake-test-key')
    assert.equal(c.options.redirect, 'error')
    assert.match(c.options.headers['Idempotency-Key'], /^[a-f0-9]{64}$/)
    const body = JSON.parse(c.options.body)
    assert.deepEqual(body.to, ['user@example.test'])
    assert.ok(!body.html.includes(config.apiKey))
    assert.match(body.text, /https:\/\/learn.example.test\/auth\?mode=/)
    assert.match(body.html, /&amp;token=/)
  }
  assert.match(JSON.parse(calls[0].options.body).text, /24 小时/)
  assert.match(JSON.parse(calls[1].options.body).text, /30 分钟/)
})
test('Resend errors never expose provider responses or keys', async () => {
  for (const transport of [
    async () => new Response('SECRET provider detail', { status: 422 }),
    async () => {
      throw Error('SECRET network detail')
    },
    async () => new Response('{}')
  ]) {
    await assert.rejects(
      createResendMailer(config, transport).sendVerificationEmail('u@example.test', 'token'),
      { message: 'mail_delivery_failed' }
    )
  }
})
test('email links require a configured trusted origin', () => {
  const keys = ['RESEND_API_KEY', 'MAIL_FROM', 'APP_BASE_URL'],
    old = keys.map(k => process.env[k])
  try {
    process.env.RESEND_API_KEY = 'test'
    process.env.MAIL_FROM = config.from
    for (const origin of [
      '',
      'http://external.test',
      'https://u:p@example.test',
      'https://example.test/path',
      'https://example.test/?q=x'
    ]) {
      process.env.APP_BASE_URL = origin
      assert.throws(mailConfig, { message: 'mail_configuration_invalid' })
    }
    process.env.APP_BASE_URL = 'http://127.0.0.1:3000'
    assert.equal(mailConfig().baseUrl, 'http://127.0.0.1:3000')
  } finally {
    keys.forEach((k, i) => {
      if (old[i] === undefined) delete process.env[k]
      else process.env[k] = old[i]
    })
  }
})
