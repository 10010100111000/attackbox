import test from 'node:test'
import assert from 'node:assert/strict'
import { normalizeAnswer, verifyQuizAnswer } from '../server/utils/quiz.ts'
import { createJiti } from 'jiti'
const { questionValues } = await createJiti(import.meta.url).import('../server/services/content-input.ts')

test('answers retain exact, trim and casefold matching semantics', () => {
  assert.equal(normalizeAnswer('  Hello World  ', 'exact'), '  Hello World  ')
  assert.equal(normalizeAnswer('  Hello World  ', 'trim'), 'Hello World')
  assert.equal(normalizeAnswer('  Hello World  ', 'trim_casefold'), 'hello world')
  for (const [submitted, expected, mode, correct] of [
    [' whoami ', 'whoami', 'exact', false],
    [' whoami ', ' whoami ', 'exact', true],
    [' whoami ', 'whoami', 'trim', true],
    ['WHOAMI', 'whoami', 'trim', false],
    [' WhoAmI ', 'whoami', 'trim_casefold', true],
    [' flag{hello} ', 'flag{hello}', 'trim', true],
    ['flag{HELLO}', 'flag{hello}', 'trim', false],
    ['wrong', 'flag{hello}', 'trim', false],
    ['0', '0', 'trim', true],
    ['', null, 'trim', false],
    ['', '', 'exact', false],
    ['  ', '  ', 'trim', false],
  ]) assert.equal(verifyQuizAnswer(submitted, expected, mode), correct, JSON.stringify([submitted, expected, mode]))
})

test('saving and replacing plain answers preserves matching rules and unset answers never pass', () => {
  const input = { promptMarkdown:'Current user?', answer:' whoami ', normalization:'trim' }
  const saved = questionValues(input)
  assert.equal(saved.answer, ' whoami ')
  assert.equal(verifyQuizAnswer('whoami', saved.answer, saved.normalization), true)
  const replaced = questionValues({ ...input, answer:'id', normalization:'exact' }, saved)
  assert.equal(verifyQuizAnswer('whoami', replaced.answer, replaced.normalization), false)
  assert.equal(verifyQuizAnswer(' id ', replaced.answer, replaced.normalization), false)
  assert.equal(verifyQuizAnswer('id', replaced.answer, replaced.normalization), true)
  assert.equal(questionValues({ ...input, answer:undefined }, replaced).answer, 'id')
  const cleared = questionValues({ ...input, answer:'  ' }, saved)
  assert.equal(cleared.answer, null)
  assert.equal(verifyQuizAnswer('', cleared.answer, cleared.normalization), false)
})
