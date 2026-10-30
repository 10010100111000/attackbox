import test from 'node:test'
import assert from 'node:assert/strict'
import { useSequentialBatch } from '../composables/useSequentialBatch.ts'

test('batch continues after item failure and stopping waits for the active mutation', async () => {
  const batch = useSequentialBatch(),
    visited = []
  await batch.run(
    ['one', 'two', 'three'],
    item => item,
    async item => {
      visited.push(item)
      if (item === 'two') throw new Error('failed')
    }
  )
  assert.deepEqual(visited, ['one', 'two', 'three'])
  assert.equal(batch.succeeded.value, 2)
  assert.equal(batch.processed.value, 3)
  assert.equal(batch.results.value.two.ok, false)
  let release
  const waiting = new Promise(resolve => {
    release = resolve
  })
  const second = batch.run(
    ['four', 'five'],
    item => item,
    async item => {
      visited.push(item)
      await waiting
    }
  )
  assert.equal(batch.running.value, true)
  batch.stop()
  assert.equal(batch.running.value, true)
  release()
  await second
  assert.equal(batch.processed.value, 1)
  assert.equal(batch.total.value, 2)
  assert.equal(batch.results.value.four.ok, true)
  assert.equal(
    batch.results.value.two.ok,
    false,
    'another operation does not erase earlier item failures'
  )
  assert.equal(batch.succeeded.value, 1)
  assert.ok(!visited.includes('five'))
  assert.equal(batch.running.value, false)
})
