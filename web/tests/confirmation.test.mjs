import test from 'node:test'
import assert from 'node:assert/strict'
import { createConfirmationController } from '../utils/confirmation.ts'

const options = { title: '删除模块？', description: '永久删除', confirmLabel: '删除模块' }
test('confirmation accepts once and restores focus before resuming the action', async () => {
  let visible
  const order = [], c = createConfirmationController(value => { visible = value })
  const promise = c.ask(options, Symbol(), () => order.push('focus')).then(value => { order.push('action'); return value })
  const id = visible.id
  c.finish(id, true)
  c.finish(id, false)
  assert.equal(visible, null)
  assert.equal(await promise, true)
  assert.deepEqual(order, ['focus', 'action'])
})
test('overlapping requests are rejected rather than queued for later execution', async () => {
  let visible
  const c = createConfirmationController(value => { visible = value })
  const first = c.ask(options, Symbol()), id = visible.id
  assert.equal(await c.ask(options, Symbol()), false)
  assert.equal(visible.id, id)
  c.cancel()
  assert.equal(await first, false)
})
test('scope cancellation never cancels another owner; stale callbacks cannot accept a newer request', async () => {
  let visible
  const c = createConfirmationController(value => { visible = value }), a = Symbol(), b = Symbol()
  const first = c.ask(options, a), oldId = visible.id
  c.cancel(b)
  assert.equal(visible.id, oldId)
  c.cancel(a)
  assert.equal(await first, false)
  const second = c.ask(options, b), newId = visible.id
  c.finish(oldId, true)
  assert.equal(visible.id, newId)
  c.finish(newId, false)
  assert.equal(await second, false)
})
