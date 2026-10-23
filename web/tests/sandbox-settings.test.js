import test from 'node:test'
import assert from 'node:assert/strict'
import { parseNodeSettings } from '../server/sandbox/settings.ts'
const baseline = { label: ' Test Node ', maxSlots: 4, ttlSeconds: 7200, idleSeconds: 1800, memoryMb: 2048, pidsMax: 512, draining: false, lockVersion: 0 }
test('node settings validate explicit fields and discard untrusted endpoint or identity overrides', () => {
  const parsed = parseNodeSettings({ ...baseline, secret: 'not-stored', publicDataUrl: 'ws://other', userId: 'other' })
  assert.deepEqual(parsed, { ...baseline, label: 'Test Node' })
})
test('node policy rejects invalid types, non-integers, empty names and idle exceeding TTL', () => {
  for (const patch of [{ maxSlots: 0 }, { maxSlots: 251 }, { memoryMb: '2048' }, { memoryMb: 1 }, { pidsMax: 1 }, { idleSeconds: 8000 }, { ttlSeconds: 59 }, { ttlSeconds: 86401 }, { draining: 'false' }, { lockVersion: -1 }, { maxSlots: 1.5 }, { label: '  ' }]) assert.throws(() => parseNodeSettings({ ...baseline, ...patch }))
  for (const value of [null, [], true]) assert.throws(() => parseNodeSettings(value))
})
