import test from 'node:test'
import assert from 'node:assert/strict'
import {
  parseRuntimePolicy,
  defaultRuntimePolicy,
} from '../shared/runtime-policy.ts'
test('global runtime defaults and strict durations', () => {
  assert.deepEqual(defaultRuntimePolicy, {
    sandboxTtlMinutes: 60,
    sandboxIdleMinutes: 10,
    targetTtlMinutes: 60,
    lockVersion: 0,
  })
  assert.deepEqual(
    parseRuntimePolicy({ ...defaultRuntimePolicy, secret: 'ignored' }),
    defaultRuntimePolicy,
  )
  for (const patch of [
    { sandboxTtlMinutes: 0 },
    { sandboxIdleMinutes: 61 },
    { targetTtlMinutes: 1441 },
    { sandboxTtlMinutes: '60' },
    { targetTtlMinutes: 1.5 },
    { lockVersion: -1 },
  ])
    assert.throws(() =>
      parseRuntimePolicy({ ...defaultRuntimePolicy, ...patch }),
    )
  assert.equal(
    parseRuntimePolicy({ ...defaultRuntimePolicy, targetTtlMinutes: 1440 })
      .targetTtlMinutes,
    1440,
  )
})
