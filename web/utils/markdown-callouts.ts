import type MarkdownIt from 'markdown-it'

const titles: Record<string, string> = {
  note: '本章学习目标',
  tip: '停下来，想一想',
  important: '重要提示',
  warning: '安全警告',
  caution: '风险警示'
}

// Transform block tokens before inline parsing. Keep the original body tokens:
// paragraphs, lists, nested quotes and fences retain MarkdownIt's block structure.
export function callouts(md: InstanceType<typeof MarkdownIt>) {
  md.core.ruler.after('block', 'callouts', state => {
    const tokens = state.tokens
    const stack: typeof tokens = []
    const ends = new Map<(typeof tokens)[number], (typeof tokens)[number]>()
    for (const token of tokens) {
      if (token.type === 'blockquote_open') stack.push(token)
      else if (token.type === 'blockquote_close') {
        const open = stack.pop()
        if (open) ends.set(open, token)
      }
    }
    for (let i = 0; i < tokens.length; i++) {
      const open = tokens[i]!,
        paragraph = tokens[i + 1],
        inline = tokens[i + 2]
      if (
        open.type !== 'blockquote_open' ||
        paragraph?.type !== 'paragraph_open' ||
        inline?.type !== 'inline'
      )
        continue
      const marker = /^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\](?:[ \t]*\n|[ \t]+|$)/i.exec(
        inline.content
      )
      if (!marker) continue
      const close = ends.get(open)
      if (!close) continue
      const type = marker[1]!.toLowerCase()
      open.type = 'callout_open'
      open.tag = 'div'
      open.attrJoin('class', `callout callout-${type}`)
      open.meta = { title: titles[type] }
      close.type = 'callout_close'
      close.tag = 'div'
      inline.content = inline.content.slice(marker[0].length)
      if (!inline.content.trim()) {
        paragraph.hidden = true
        tokens[i + 3]!.hidden = true
      }
    }
  })
  md.renderer.rules.callout_open = (tokens, index, options, _env, renderer) => {
    const title = tokens[index]!.meta?.title
    return `${renderer.renderToken(tokens, index, options)}<strong>${md.utils.escapeHtml(typeof title === 'string' ? title : '提示')}</strong>\n`
  }
}
