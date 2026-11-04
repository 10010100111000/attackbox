export const SITE_NAME = 'AttackBox'
export const DEFAULT_SITE_ORIGIN = 'https://attackbox.local'

export const DEFAULT_SEO_TITLE = 'AttackBox — 理解原理，亲手验证 | 网络安全学习与实战靶场平台'
export const DEFAULT_SEO_DESCRIPTION =
  '理解原理，亲手验证。在 AttackBox 系统学习网络安全、Web 渗透测试、Linux 基础与夺旗挑战（CTF），通过交互式在线章节与真实容器靶场环境提升安全实战能力。'

export const DEFAULT_SEO_KEYWORDS = [
  'AttackBox',
  '网络安全学习',
  '网络安全靶场',
  '渗透测试教程',
  'Web安全实战',
  'CTF训练平台',
  'Linux安全入门',
  '在线实验环境',
  '信息安全靶场',
  '攻防演练实践'
].join(',')

const DIFFICULTY_SEO_LABELS: { [key: string]: string } = {
  beginner: '基础入门',
  intermediate: '中级进阶',
  advanced: '高阶实战'
}

export function difficultySeoLabel(difficulty?: string | null): string {
  if (!difficulty) return '学习模块'
  return DIFFICULTY_SEO_LABELS[difficulty] || '学习模块'
}

export function normalizeSiteOrigin(
  configuredOrigin?: string | null,
  requestOrigin?: string | null
): string {
  for (const candidate of [configuredOrigin, requestOrigin, DEFAULT_SITE_ORIGIN]) {
    if (!candidate || typeof candidate !== 'string') continue
    const trimmed = candidate.trim()
    if (!trimmed) continue
    try {
      const parsed = new URL(trimmed)
      if (parsed.protocol === 'http:' || parsed.protocol === 'https:') {
        return parsed.origin
      }
    } catch {
      // Ignore malformed origin candidate and fall back to next option.
    }
  }
  return DEFAULT_SITE_ORIGIN
}

export function canonicalUrl(origin: string, pathname: string): string {
  const cleanOrigin = normalizeSiteOrigin(origin)
  const rawPath = (pathname || '/').split(/[?#]/)[0] || '/'
  const normalizedPath = rawPath.startsWith('/') ? rawPath : `/${rawPath}`
  const collapsed = normalizedPath.length > 1 ? normalizedPath.replace(/\/+$/, '') : '/'
  return `${cleanOrigin}${collapsed}`
}

export function stripMarkdownToText(markdown?: string | null): string {
  if (!markdown || typeof markdown !== 'string') return ''
  return markdown
    .replace(/^---\r?\n[\s\S]*?\r?\n---\r?\n?/, '')
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/~~~[\s\S]*?~~~/g, ' ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/!\[([^\]]*)\]\([^)]+\)/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .replace(/\x3cscript\b[^\x3e]*\x3e[\s\S]*?\x3c\/script\x3e/gi, ' ')
    .replace(/\x3cstyle\b[^\x3e]*\x3e[\s\S]*?\x3c\/style\x3e/gi, ' ')
    .replace(/\x3c[^\x3e]+\x3e/g, ' ')
    .replace(/^[#>*~_+-]+\s*/gm, '')
    .replace(/\|/g, ' ')
    .replace(/[#*~_>`]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

export function extractSeoExcerpt(markdown?: string | null, maxLength = 150): string {
  const plain = stripMarkdownToText(markdown)
  if (!plain) return ''
  const limit = Math.max(20, maxLength)
  if (plain.length <= limit) return plain
  const slice = plain.slice(0, limit)
  const boundary = Math.max(
    slice.lastIndexOf('。'),
    slice.lastIndexOf('！'),
    slice.lastIndexOf('？'),
    slice.lastIndexOf('；'),
    slice.lastIndexOf('. ')
  )
  if (boundary >= Math.floor(limit * 0.6)) {
    return slice.slice(0, boundary + 1).trim()
  }
  return `${slice.trim()}…`
}

export type KeywordInput = string | null | undefined | readonly (string | null | undefined)[]

export function buildKeywords(...parts: KeywordInput[]): string {
  const seen: string[] = []
  for (const part of parts) {
    const items = Array.isArray(part) ? part : [part]
    for (const item of items) {
      if (!item || typeof item !== 'string') continue
      for (const token of item.split(/[,，、]/)) {
        const cleaned = token.trim()
        if (cleaned && cleaned.length <= 40 && !seen.includes(cleaned)) {
          seen.push(cleaned)
        }
      }
    }
  }
  return seen.slice(0, 18).join(',')
}

export function escapeXml(value: string): string {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/\x3c/g, '&lt;')
    .replace(/\x3e/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;')
}

export function serializeJsonLd(data: object): string {
  return JSON.stringify(data)
    .replace(/\x3c/g, '\\u003c')
    .replace(/\x3e/g, '\\u003e')
    .replace(/&/g, '\\u0026')
    .replace(/\u2028/g, '\\u2028')
    .replace(/\u2029/g, '\\u2029')
}
