import test from 'node:test'
import assert from 'node:assert/strict'
import { createJiti } from 'jiti'
const { feedbackInput, feedbackStatus, feedbackId } = await createJiti(import.meta.url).import('../server/utils/feedback.ts')

test('feedback validates bounded text and known categories, preserving plain text', () => {
  for (const category of ['bug','content','suggestion']) {
    assert.deepEqual(feedbackInput({ category, body:'  <script>text only</script>\n第二行  ' }), {
      category, body:'<script>text only</script>\n第二行', sourcePath:null,
    })
  }
  for (const value of [null, {}, { category:'other',body:'valid' }, { category:'bug',body:'   ' },
    { category:'bug',body:42 }, { category:'bug',body:'x'.repeat(4001) }, { category:'bug',body:'a\0b' }]) {
    assert.throws(() => feedbackInput(value), error => error.statusCode === 400)
  }
})

test('feedback context keeps only a local pathname, excluding query strings and fragments', () => {
  assert.equal(feedbackInput({ category:'content',body:'错字',sourcePath:'/learn/modules/example/chapter?token=private#section' }).sourcePath, '/learn/modules/example/chapter')
  assert.equal(feedbackInput({ category:'bug',body:'test',sourcePath:'' }).sourcePath, null)
  for (const sourcePath of ['https://evil.test','//evil.test','/\\evil.test','/%2fevil.test','/%5cevil.test','/%00','/%zz','/a b','/'.repeat(501),42]) {
    assert.throws(() => feedbackInput({ category:'bug',body:'test',sourcePath }), error => error.statusCode === 400)
  }
})

test('feedback updates accept only explicit statuses and UUID identifiers', () => {
  for (const status of ['new','in_progress','resolved']) assert.equal(feedbackStatus(status), status)
  for (const status of ['all','published','',null,1]) assert.throws(() => feedbackStatus(status), error => error.statusCode === 400)
  const id = 'ab012345-6789-4abc-8def-0123456789ab'
  assert.equal(feedbackId(id), id)
  for (const value of [undefined,'../user',id + '?status=new','not-a-uuid']) assert.throws(() => feedbackId(value), error => error.statusCode === 400)
})
