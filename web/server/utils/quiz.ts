export type NormalizationKind = 'exact' | 'trim' | 'trim_casefold'

/**
 * 按照题目设定的规则规范化答案文本
 */
export function normalizeAnswer(raw: string, mode: NormalizationKind = 'trim'): string {
  const str = String(raw ?? '')
  switch (mode) {
    case 'exact':
      return str
    case 'trim_casefold':
      return str.trim().toLowerCase()
    case 'trim':
    default:
      return str.trim()
  }
}

/** Compare stored answers on the server using the question's matching rule. */
export function verifyQuizAnswer(
  userAnswer: string,
  expectedAnswer: string | null,
  mode: NormalizationKind = 'trim'
): boolean {
  if (expectedAnswer === null) return false
  const expected = normalizeAnswer(expectedAnswer, mode)
  return expected.length > 0 && normalizeAnswer(userAnswer, mode) === expected
}
