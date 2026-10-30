import type { Token } from 'markdown-it'

function slugify(text: string): string {
  return (
    text
      .toLowerCase()
      .trim()
      .replace(/[^\p{L}\p{N}\s_-]/gu, '')
      .replace(/[\s_-]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'section'
  )
}

/** Assign IDs before rendering so separately rendered token ranges keep global anchors. */
export function markdownHeadings(tokens: Token[]) {
  const used = new Set<string>()
  return tokens.flatMap((token, index) => {
    if (token.type !== 'heading_open') return []
    const level = Number(token.tag.slice(1))
    const nextToken = tokens[index + 1]
    const titleText =
      nextToken?.type === 'inline'
        ? (nextToken.children || [])
            .map(child =>
              ['text', 'code_inline', 'image'].includes(child.type)
                ? child.content
                : ['softbreak', 'hardbreak'].includes(child.type)
                  ? ' '
                  : ''
            )
            .join('')
        : ''

    const base = slugify(titleText)
    let id = base,
      suffix = 1
    while (used.has(id)) id = `${base}-${suffix++}`
    used.add(id)
    token.attrSet('id', id)
    return [{ id, text: titleText, level }]
  })
}
