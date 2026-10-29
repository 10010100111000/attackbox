// web/tests/auth-crypto.test.js
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  hashPassword,
  verifyPassword,
  generateSecureToken,
  hashToken
} from '../server/utils/auth.ts'

test('hashPassword creates a valid Argon2id hash and verifyPassword validates it', async () => {
  const password = 'SuperSecretPassword123!'
  const hash = await hashPassword(password)

  assert.ok(hash.startsWith('$argon2id$'), 'Hash should be Argon2id')

  const isValid = await verifyPassword(hash, password)
  assert.equal(isValid, true, 'Correct password should verify')

  const isInvalid = await verifyPassword(hash, 'WrongPassword')
  assert.equal(isInvalid, false, 'Wrong password should fail')
})

test('generateSecureToken produces 64-char hex string (32 bytes entropy)', () => {
  const token1 = generateSecureToken()
  const token2 = generateSecureToken()

  assert.equal(typeof token1, 'string')
  assert.equal(token1.length, 64)
  assert.notEqual(token1, token2)
})

test('hashToken produces 32-byte Buffer SHA-256 digest', () => {
  const token = generateSecureToken()
  const digest = hashToken(token)

  assert.ok(Buffer.isBuffer(digest))
  assert.equal(digest.length, 32)

  const digest2 = hashToken(token)
  assert.deepEqual(digest, digest2, 'Same token must produce identical digest')
})
