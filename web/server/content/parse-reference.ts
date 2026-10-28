import path from 'node:path'
import MarkdownIt from 'markdown-it'
import { parseDocument } from 'yaml'
import { ContentFormatError, type ContentImage } from './parse-module.ts'
export function parseReferenceMarkdown(sourcePath: string, source: string) {
  function fail(message: string): never { throw new ContentFormatError([{ file: sourcePath, line: 1, message }]) }
  if (!sourcePath.endsWith('.md') || path.posix.isAbsolute(sourcePath) || sourcePath.includes('\\') || sourcePath.split('/').some(p => !p || p === '.' || p === '..')) fail('需要仓库内的相对 .md 文件路径')
  if (Buffer.byteLength(source) > 2 * 1024 * 1024) fail('单篇资料不能超过 2 MiB')
  const title = path.posix.basename(sourcePath, '.md').trim()
  if (!title || title.length > 160) fail('文件名作为资料名称，长度为 1–160 字符')
  let bodyMarkdown = source.replace(/^\uFEFF/, '').replace(/\r\n?/g, '\n'), summary = ''
  if (bodyMarkdown.startsWith('---\n')) {
    const lines = bodyMarkdown.split('\n'), end = lines.indexOf('---', 1)
    if (end < 0) fail('配置头缺少结束的 ---')
    const doc = parseDocument(lines.slice(1, end).join('\n'), { uniqueKeys: true, merge: false, logLevel: 'silent' })
    if (doc.errors.length || doc.warnings.length) fail('资料配置头的 YAML 格式无效')
    let header: unknown
    try { header = doc.toJS({ maxAliasCount: 0 }) } catch { fail('配置头不支持 YAML 别名') }
    if (header !== null && (typeof header !== 'object' || Array.isArray(header))) fail('配置头应为字段配置')
    const fields = (header || {}) as Record<string, unknown>
    if (Object.keys(fields).some(key => key !== 'summary')) fail('资料配置头仅支持 summary 字段')
    if (fields.summary !== undefined && (typeof fields.summary !== 'string' || fields.summary.length > 500)) fail('summary 最多 500 字符')
    summary = (fields.summary as string || '').trim()
    bodyMarkdown = lines.slice(end + 1).join('\n').trim()
  }
  if (!bodyMarkdown.trim()) fail('资料正文不能为空')
  const md = new MarkdownIt({ html: false }), images: ContentImage[] = []
  const walk = (tokens: ReturnType<typeof md.parse>) => {
    for (const token of tokens) {
      if (token.type === 'image') images.push({ file: sourcePath, line: 1, href: String(token.attrGet('src') || '') })
      if (token.children) walk(token.children)
    }
  }
  walk(md.parse(bodyMarkdown, {}))
  if (new Set(images.map(image => image.href)).size > 64) fail('单篇资料最多引用 64 张图片')
  return { title, summary, bodyMarkdown, images }
}
