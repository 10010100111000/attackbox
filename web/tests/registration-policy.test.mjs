import test from 'node:test'
import assert from 'node:assert/strict'
import { createJiti } from 'jiti'
import { defaultRegistrationDomains, registrationEmailAllowed } from '../shared/registration.ts'
const { registrationDomains } = await createJiti(import.meta.url).import(
  '../server/utils/registration-policy.ts'
)
test('default registration policy allows selected providers with exact domain matching', () => {
  const domains = registrationDomains({})
  assert.deepEqual(domains, [...defaultRegistrationDomains])
  for (const email of [
    'user@gmail.com',
    ' USER+notes@GMAIL.COM ',
    'u@qq.com',
    'u@163.com',
    'u@outlook.com',
    'u@icloud.com'
  ])
    assert.equal(registrationEmailAllowed(email, domains), true, email)
  for (const email of [
    'u@mailinator.com',
    'u@10minutemail.com',
    'u@gmail.com.evil.test',
    'u@sub.gmail.com',
    'u@gmaıl.com',
    'u@Ｇｍａｉｌ.com',
    'u@gmail.com.',
    'gmail.com@temporary.test',
    'u@@gmail.com',
    'u@\ngmail.com',
    'u@outlook.example',
    'u@attackbox.org'
  ])
    assert.equal(registrationEmailAllowed(email, domains), false, email)
})
test('custom domains replace defaults and invalid or empty policies never allow everyone', () => {
  const domains = registrationDomains({
    AUTH_REGISTRATION_EMAIL_DOMAINS: 'EXAMPLE.ORG, gmail.com,example.org'
  })
  assert.deepEqual(domains, ['example.org', 'gmail.com'])
  assert.equal(registrationEmailAllowed('u@example.org', domains), true)
  assert.equal(registrationEmailAllowed('u@qq.com', domains), false)
  for (const value of [
    '',
    ',',
    '*',
    '*.gmail.com',
    'gmail.com,',
    'https://gmail.com',
    '@gmail.com',
    'gmail.com/',
    'gmail..com',
    '-bad.com'
  ])
    assert.throws(
      () => registrationDomains({ AUTH_REGISTRATION_EMAIL_DOMAINS: value }),
      e => e.statusCode === 503
    )
})
