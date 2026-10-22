// web/tests/quiz-crypto.test.js
import test from 'node:test';
import assert from 'node:assert/strict';
import { normalizeAnswer, hashQuizAnswer, verifyQuizAnswer } from '../server/utils/quiz.ts';

test('normalizeAnswer rules', () => {
  assert.equal(normalizeAnswer('  hello World  ', 'trim'), 'hello World');
  assert.equal(normalizeAnswer('  hello World  ', 'trim_casefold'), 'hello world');
  assert.equal(normalizeAnswer('  hello World  ', 'exact'), '  hello World  ');
  assert.equal(normalizeAnswer('whoami', 'trim'), 'whoami');
});

test('hashQuizAnswer produces valid 32-byte HMAC', () => {
  const hmac1 = hashQuizAnswer('whoami', 'trim');
  assert.ok(Buffer.isBuffer(hmac1));
  assert.equal(hmac1.length, 32);

  // 相同输入产生相同摘要
  const hmac2 = hashQuizAnswer('  whoami  ', 'trim');
  assert.deepEqual(hmac1, hmac2);

  // 大小写不同在 trim 模式下摘要不同
  const hmacUpper = hashQuizAnswer('WHOAMI', 'trim');
  assert.notDeepEqual(hmac1, hmacUpper);

  // 在 trim_casefold 模式下摘要相同
  const hmacCasefold1 = hashQuizAnswer('WhoAmI', 'trim_casefold');
  const hmacCasefold2 = hashQuizAnswer('  whoami  ', 'trim_casefold');
  assert.deepEqual(hmacCasefold1, hmacCasefold2);
});

test('verifyQuizAnswer timing-safe validation', () => {
  const answer = 'flag{welcome_to_attackbox}';
  const hmac = hashQuizAnswer(answer, 'trim');

  // 正确答案
  assert.equal(verifyQuizAnswer('flag{welcome_to_attackbox}', hmac, 'trim'), true);
  assert.equal(verifyQuizAnswer('  flag{welcome_to_attackbox}  ', hmac, 'trim'), true);

  // 错误答案
  assert.equal(verifyQuizAnswer('wrong_flag', hmac, 'trim'), false);

  // 非法长度 HMAC 保护
  assert.equal(verifyQuizAnswer(answer, Buffer.alloc(16), 'trim'), false);
});
