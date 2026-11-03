import type MarkdownIt from 'markdown-it'

interface FootnoteEnv {
  footnotes?: {
    defs: Map<string, string>
    order: string[]
  }
}

function installTaskLists(md: InstanceType<typeof MarkdownIt>) {
  md.core.ruler.after('inline', 'task_lists', state => {
    const tokens = state.tokens
    const listStack: number[] = []
    for (let i = 0; i < tokens.length; i++) {
      const token = tokens[i]!
      if (token.type === 'bullet_list_open' || token.type === 'ordered_list_open') {
        listStack.push(i)
        continue
      }
      if (token.type === 'bullet_list_close' || token.type === 'ordered_list_close') {
        listStack.pop()
        continue
      }
      if (
        token.type === 'inline' &&
        i >= 2 &&
        tokens[i - 1]?.type === 'paragraph_open' &&
        tokens[i - 2]?.type === 'list_item_open' &&
        token.children?.length &&
        token.children[0]?.type === 'text'
      ) {
        const first = token.children[0]
        const match = /^\[([ xX])\]\s+/.exec(first.content)
        if (!match) continue
        const checked = match[1]!.toLowerCase() === 'x'
        first.content = first.content.slice(match[0].length)
        token.content = token.content.slice(match[0].length)
        const checkbox = new state.Token('task_checkbox', 'input', 0)
        checkbox.meta = { checked }
        token.children.unshift(checkbox)
        tokens[i - 2]!.attrJoin('class', 'task-list-item')
        const parentListIdx = listStack.at(-1)
        if (parentListIdx !== undefined) {
          tokens[parentListIdx]!.attrJoin('class', 'contains-task-list')
        }
      }
    }
  })

  md.renderer.rules.task_checkbox = (tokens, idx) => {
    const checked = Boolean(tokens[idx]?.meta?.checked)
    return `<input class="task-list-item-checkbox" type="checkbox" disabled${checked ? ' checked' : ''}> `
  }
}

function installInlinePair(
  md: InstanceType<typeof MarkdownIt>,
  name: string,
  tag: string,
  markerChar: number,
  doubleMarker: boolean
) {
  md.inline.ruler.before('emphasis', name, (state, silent) => {
    const start = state.pos
    const max = state.posMax
    const src = state.src
    if (src.charCodeAt(start) !== markerChar) return false
    if (doubleMarker) {
      if (start + 3 >= max || src.charCodeAt(start + 1) !== markerChar) return false
    } else {
      if (start + 2 >= max || src.charCodeAt(start + 1) === markerChar) return false
    }
    const step = doubleMarker ? 2 : 1
    const contentStart = start + step
    if (src.charCodeAt(contentStart) === 0x20 || src.charCodeAt(contentStart) === 0x09) return false

    let pos = contentStart
    while (pos < max) {
      if (src.charCodeAt(pos) === 0x5c /* \ */) {
        pos += 2
        continue
      }
      if (
        src.charCodeAt(pos) === markerChar &&
        (!doubleMarker || (pos + 1 < max && src.charCodeAt(pos + 1) === markerChar))
      ) {
        if (!doubleMarker && pos + 1 < max && src.charCodeAt(pos + 1) === markerChar) {
          pos += 2
          continue
        }
        break
      }
      if (!doubleMarker && (src.charCodeAt(pos) === 0x20 || src.charCodeAt(pos) === 0x0a)) {
        return false
      }
      pos++
    }
    if (pos >= max || pos === contentStart) return false
    if (src.charCodeAt(pos - 1) === 0x20 || src.charCodeAt(pos - 1) === 0x09) return false

    if (!silent) {
      state.pos = contentStart
      state.posMax = pos
      state.push(`${name}_open`, tag, 1)
      state.md.inline.tokenize(state)
      state.push(`${name}_close`, tag, -1)
    }
    state.pos = pos + step
    state.posMax = max
    return true
  })
}

function installFootnotes(md: InstanceType<typeof MarkdownIt>) {
  md.block.ruler.before('reference', 'footnote_def', (state, startLine, _endLine, silent) => {
    const start = state.bMarks[startLine]! + state.tShift[startLine]!
    const max = state.eMarks[startLine]!
    const line = state.src.slice(start, max)
    const match = /^\[\^([^\]\s]+)\]:\s*(.+)$/.exec(line)
    if (!match) return false
    if (silent) return true
    const env = state.env as FootnoteEnv
    env.footnotes ||= { defs: new Map(), order: [] }
    env.footnotes.defs.set(match[1]!, match[2]!.trim())
    state.line = startLine + 1
    return true
  })

  md.inline.ruler.after('image', 'footnote_ref', (state, silent) => {
    const start = state.pos
    const max = state.posMax
    if (start + 3 >= max) return false
    if (state.src.charCodeAt(start) !== 0x5b /* [ */ || state.src.charCodeAt(start + 1) !== 0x5e /* ^ */)
      return false
    const match = /^\[\^([^\]\s]+)\]/.exec(state.src.slice(start, max))
    if (!match) return false
    const label = match[1]!
    const env = state.env as FootnoteEnv
    if (!env.footnotes?.defs.has(label)) return false
    if (!silent) {
      let idx = env.footnotes.order.indexOf(label)
      if (idx === -1) {
        idx = env.footnotes.order.length
        env.footnotes.order.push(label)
      }
      const token = state.push('footnote_ref', '', 0)
      token.meta = { index: idx + 1, label }
    }
    state.pos = start + match[0].length
    return true
  })

  md.core.ruler.after('inline', 'footnote_tail', state => {
    const env = state.env as FootnoteEnv
    if (!env.footnotes || !env.footnotes.order.length) return
    const open = new state.Token('footnote_block_open', 'section', 1)
    state.tokens.push(open)
    for (let i = 0; i < env.footnotes.order.length; i++) {
      const label = env.footnotes.order[i]!
      const raw = env.footnotes.defs.get(label) || ''
      const itemOpen = new state.Token('footnote_item_open', 'li', 1)
      itemOpen.meta = { index: i + 1 }
      state.tokens.push(itemOpen)
      const inline = new state.Token('inline', '', 0)
      inline.content = raw
      inline.children = []
      state.md.inline.parse(raw, state.md, state.env, inline.children)
      state.tokens.push(inline)
      const itemClose = new state.Token('footnote_item_close', 'li', -1)
      itemClose.meta = { index: i + 1 }
      state.tokens.push(itemClose)
    }
    const close = new state.Token('footnote_block_close', 'section', -1)
    state.tokens.push(close)
  })

  md.renderer.rules.footnote_ref = (tokens, idx) => {
    const index = Number(tokens[idx]?.meta?.index || 1)
    return `<sup class="footnote-ref"><a href="#fn${index}" id="fnref${index}">[${index}]</a></sup>`
  }
  md.renderer.rules.footnote_block_open = () =>
    '<section class="footnotes" aria-label="脚注"><ol class="footnotes-list">\n'
  md.renderer.rules.footnote_block_close = () => '</ol></section>\n'
  md.renderer.rules.footnote_item_open = (tokens, idx) => {
    const index = Number(tokens[idx]?.meta?.index || 1)
    return `<li id="fn${index}" class="footnote-item">`
  }
  md.renderer.rules.footnote_item_close = (tokens, idx) => {
    const index = Number(tokens[idx]?.meta?.index || 1)
    return ` <a href="#fnref${index}" class="footnote-backref" aria-label="返回正文">↩</a></li>\n`
  }
}

export function markdownExtensions(md: InstanceType<typeof MarkdownIt>) {
  installTaskLists(md)
  installInlinePair(md, 'mark', 'mark', 0x3d /* = */, true)
  installInlinePair(md, 'sub', 'sub', 0x7e /* ~ */, false)
  installInlinePair(md, 'sup', 'sup', 0x5e /* ^ */, false)
  installFootnotes(md)
}
