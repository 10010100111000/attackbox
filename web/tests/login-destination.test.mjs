import test from 'node:test'
import assert from 'node:assert/strict'
import { createJiti } from 'jiti'
const { loginDestination } = await createJiti(import.meta.url).import('../utils/login-destination.ts')
test('login returns to internal pages and defaults safely for external, malformed and looping destinations', () => {
  for (const path of ['/learn','/me','/admin/modules','/learn/modules/linux/intro?mode=read#shell','/challenges/test']) assert.equal(loginDestination(path),path)
  for (const path of [undefined,null,['/me'],'https://evil.test','//evil.test','/\\evil.test','/auth','/auth?returnTo=/auth','/a/../auth','/%2f%2fevil.test','/%5cevil.test','/bad%zz','/a\n/evil']) assert.equal(loginDestination(path),'/learn')
})
