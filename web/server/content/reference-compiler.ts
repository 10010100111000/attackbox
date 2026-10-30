import { createMarkdownRenderer, type MarkdownHeading } from '../../utils/markdown.ts'

const renderer = createMarkdownRenderer()
const targetSize = 24000
/** Compile at import time, splitting only between top-level Markdown blocks:
 * fences, lists, callouts and cross-section reference links retain their semantics. */
export function compileReference(source: string, imageUrls: Record<string, string> = {}) {
  const headings: MarkdownHeading[] = []
  const tokens = renderer.parse(source, { headings })
  const lines = source.split('\n')
  const offsets = [0]
  for (const line of lines) offsets.push(offsets.at(-1)! + line.length + 1)
  const blocks = tokens.flatMap((token, index) =>
    token.level === 0 && token.nesting !== -1 && token.map
      ? [{ token, index, offset: offsets[token.map[0]]! }]
      : []
  )
  const topLevel = blocks.reduce(
    (level, b) =>
      b.token.type === 'heading_open' ? Math.min(level, Number(b.token.tag.slice(1))) : level,
    Infinity
  )
  const byId = new Map(headings.map(h => [h.id, h]))
  const ranges: { start: number; end: number; title: string }[] = []
  let start = 0,
    offset = 0,
    title = '开始阅读',
    hasHeading = false
  for (const block of blocks) {
    const heading = block.token.type === 'heading_open'
    const boundary = heading && Number(block.token.tag.slice(1)) === topLevel
    if (block.index > start && ((boundary && hasHeading) || block.offset - offset >= targetSize)) {
      ranges.push({ start, end: block.index, title })
      hasHeading = false
      start = block.index
      offset = block.offset
      if (!heading && !title.endsWith('（续）')) title += '（续）'
    }
    if (heading && !hasHeading) title = byId.get(String(block.token.attrGet('id')))!.text
    if (heading) hasHeading = true
  }
  ranges.push({ start, end: tokens.length, title })
  const chunks = ranges.map((range, index) => ({
    index,
    title: range.title,
    html: renderer.renderer.render(tokens.slice(range.start, range.end), renderer.options, {
      imageUrls
    })
  }))
  const outline = ranges.flatMap((range, index) =>
    tokens
      .slice(range.start, range.end)
      .filter(t => t.type === 'heading_open')
      .map(t => ({ ...byId.get(String(t.attrGet('id')))!, chunk: index }))
  )
  return { chunks, outline }
}
