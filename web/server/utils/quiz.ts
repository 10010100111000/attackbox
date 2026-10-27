import crypto from 'node:crypto';

export type NormalizationKind = 'exact' | 'trim' | 'trim_casefold';

function answerSecret(): string {
  const secret = process.env.QUIZ_ANSWER_HMAC_SECRET;
  if (secret && Buffer.byteLength(secret) >= 32) return secret;
  if (process.env.NODE_ENV === 'production') throw new Error('QUIZ_ANSWER_HMAC_SECRET must contain at least 32 bytes');
  return 'attackbox-quiz-secret-dev-2026-key1';
}

/**
 * 按照题目设定的规则规范化答案文本
 */
export function normalizeAnswer(raw: string, mode: NormalizationKind = 'trim'): string {
  const str = String(raw ?? '');
  switch (mode) {
    case 'exact':
      return str;
    case 'trim_casefold':
      return str.trim().toLowerCase();
    case 'trim':
    default:
      return str.trim();
  }
}

/**
 * 计算题目标准答案的 HMAC-SHA256 (32 字节 Buffer)
 */
export function hashQuizAnswer(
  plainAnswer: string,
  mode: NormalizationKind = 'trim',
  secret: string = answerSecret()
): Buffer {
  const normalized = normalizeAnswer(plainAnswer, mode);
  return crypto.createHmac('sha256', secret).update(normalized, 'utf-8').digest();
}

/**
 * 使用常量时间比较用户答案 HMAC 与存储的标准答案 HMAC
 */
export function verifyQuizAnswer(
  userAnswer: string,
  expectedHmac: Buffer,
  mode: NormalizationKind = 'trim',
  secret: string = answerSecret()
): boolean {
  if (!expectedHmac || expectedHmac.length !== 32) {
    return false;
  }
  const computedHmac = hashQuizAnswer(userAnswer, mode, secret);
  try {
    return crypto.timingSafeEqual(computedHmac, expectedHmac);
  } catch {
    return false;
  }
}
