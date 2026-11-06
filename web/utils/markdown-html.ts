const VOID_TAGS = new Set([
  'area',
  'base',
  'br',
  'col',
  'embed',
  'hr',
  'img',
  'input',
  'link',
  'meta',
  'param',
  'source',
  'track',
  'wbr'
])

const ESCAPED_BLOCK_TAGS = new Set([
  'script',
  'style',
  'object',
  'embed',
  'applet',
  'form',
  'input',
  'button',
  'textarea',
  'select',
  'option',
  'meta',
  'link',
  'base',
  'head',
  'html',
  'body',
  'frame',
  'frameset'
])

const ALLOWED_TAGS = new Set([
  'a',
  'abbr',
  'article',
  'aside',
  'audio',
  'b',
  'blockquote',
  'br',
  'caption',
  'cite',
  'code',
  'col',
  'colgroup',
  'dd',
  'del',
  'details',
  'dfn',
  'div',
  'dl',
  'dt',
  'em',
  'figcaption',
  'figure',
  'footer',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'header',
  'hr',
  'i',
  'iframe',
  'img',
  'ins',
  'kbd',
  'li',
  'mark',
  'ol',
  'p',
  'pre',
  'q',
  'rp',
  'rt',
  'ruby',
  's',
  'samp',
  'section',
  'small',
  'source',
  'span',
  'strong',
  'sub',
  'summary',
  'sup',
  'table',
  'tbody',
  'td',
  'tfoot',
  'th',
  'thead',
  'time',
  'tr',
  'track',
  'u',
  'ul',
  'var',
  'video',
  'wbr'
])

const GLOBAL_ATTRS = new Set(['id', 'class', 'title', 'lang', 'dir', 'style', 'align', ' role', 'aria-label', 'aria-hidden'])

const TAG_ATTRS: Record<string, Set<string>> = {
  a: new Set(['href', 'target', 'rel', 'download']),
  img: new Set(['src', 'alt', 'width', 'height', 'loading']),
  iframe: new Set([
    'src',
    'width',
    'height',
    'title',
    'allow',
    'allowfullscreen',
    'frameborder',
    'scrolling',
    'loading',
    'referrerpolicy'
  ]),
  video: new Set([
    'src',
    'poster',
    'width',
    'height',
    'controls',
    'autoplay',
    'loop',
    'muted',
    'playsinline',
    'preload'
  ]),
  audio: new Set(['src', 'controls', 'autoplay', 'loop', 'muted', 'preload']),
  source: new Set(['src', 'type']),
  track: new Set(['src', 'kind', 'srclang', 'label', 'default']),
  details: new Set(['open']),
  ol: new Set(['start', 'type', 'reversed']),
  li: new Set(['value']),
  table: new Set(['width']),
  th: new Set(['colspan', 'rowspan', 'scope', 'width', 'align']),
  td: new Set(['colspan', 'rowspan', 'width', 'align']),
  col: new Set(['span', 'width']),
  colgroup: new Set(['span', 'width']),
  time: new Set(['datetime']),
  blockquote: new Set(['cite']),
  q: new Set(['cite'])
}

const BOOLEAN_ATTRS = new Set([
  'allowfullscreen',
  'autoplay',
  'controls',
  'default',
  'loop',
  'muted',
  'open',
  'playsinline',
  'reversed'
])

const SAFE_STYLE_PROPS = new Set([
  'width',
  'max-width',
  'min-width',
  'height',
  'max-height',
  'min-height',
  'aspect-ratio',
  'margin',
  'margin-top',
  'margin-bottom',
  'margin-left',
  'margin-right',
  'padding',
  'padding-top',
  'padding-bottom',
  'padding-left',
  'padding-right',
  'border',
  'border-radius',
  'background',
  'background-color',
  'color',
  'font-size',
  'font-weight',
  'line-height',
  'text-align',
  'display',
  'flex',
  'flex-direction',
  'align-items',
  'justify-content',
  'gap',
  'grid-template-columns',
  'overflow'
])

export function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

export function isSafeLinkUrl(href: string): boolean {
  const trimmed = href.trim()
  return (
    /^https?:\/\//i.test(trimmed) ||
    /^mailto:/i.test(trimmed) ||
    /^#/i.test(trimmed) ||
    /^\/(?!\/)/i.test(trimmed)
  )
}

export function normalizeMediaUrl(
  rawUrl: string,
  imageUrls?: Record<string, string>
): { url: string; external: boolean } | null {
  const trimmed = rawUrl.trim()
  if (!trimmed) return null
  if (imageUrls && Object.hasOwn(imageUrls, trimmed)) {
    return { url: imageUrls[trimmed]!, external: /^https?:\/\//i.test(imageUrls[trimmed]!) }
  }
  if (/^\/\/[a-zA-Z0-9.-]+\//.test(trimmed)) {
    return { url: `https:${trimmed}`, external: true }
  }
  if (/^https?:\/\//i.test(trimmed)) {
    return { url: trimmed, external: true }
  }
  if (/^\/(?!\/)/.test(trimmed)) {
    return { url: trimmed, external: false }
  }
  if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(trimmed) && !trimmed.startsWith('..')) {
    return { url: trimmed, external: false }
  }
  return null
}

function sanitizeInlineStyle(styleValue: string): string {
  const declarations = styleValue.split(';')
  const kept: string[] = []
  for (const decl of declarations) {
    const colon = decl.indexOf(':')
    if (colon <= 0) continue
    const prop = decl.slice(0, colon).trim().toLowerCase()
    const val = decl.slice(colon + 1).trim()
    if (!SAFE_STYLE_PROPS.has(prop)) continue
    if (
      /url\s*\(|expression\s*\(|javascript:|vbscript:|data:|-moz-binding|behavior\s*:/i.test(
        val
      )
    )
      continue
    kept.push(`${prop}: ${val}`)
  }
  return kept.join('; ')
}

function parseAttributes(attrString: string): Array<{ name: string; value: string }> {
  const attrs: Array<{ name: string; value: string }> = []
  const attrRegex =
    /([a-zA-Z_:][a-zA-Z0-9_.:-]*)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+)))?/g
  let match: RegExpExecArray | null
  while ((match = attrRegex.exec(attrString)) !== null) {
    const name = match[1]!.toLowerCase()
    const value = match[2] ?? match[3] ?? match[4] ?? ''
    attrs.push({ name, value })
  }
  return attrs
}

function sanitizeSingleTag(
  fullTag: string,
  imageUrls?: Record<string, string>
): string {
  const match = /^<(\/?)([a-zA-Z][a-zA-Z0-9:-]*)([\s\S]*?)(\/?)>$/.exec(fullTag)
  if (!match) return escapeHtml(fullTag)

  const isClosing = match[1] === '/'
  const tag = match[2]!.toLowerCase()
  const rawAttrs = match[3] || ''

  if (ESCAPED_BLOCK_TAGS.has(tag) || !ALLOWED_TAGS.has(tag)) {
    return escapeHtml(fullTag)
  }

  if (isClosing) {
    return VOID_TAGS.has(tag) ? '' : `</${tag}>`
  }

  const parsedAttrs = parseAttributes(rawAttrs)
  const allowedForTag = TAG_ATTRS[tag]
  const sanitizedAttrs = new Map<string, string>()

  for (const { name, value } of parsedAttrs) {
    if (name.startsWith('on') || name === 'srcdoc') continue
    if (!GLOBAL_ATTRS.has(name) && !allowedForTag?.has(name)) continue

    if (name === 'style') {
      const cleanStyle = sanitizeInlineStyle(value)
      if (cleanStyle) sanitizedAttrs.set('style', cleanStyle)
      continue
    }

    if (name === 'href') {
      const safeHref = isSafeLinkUrl(value) ? value.trim() : '#unsafe-link-blocked'
      sanitizedAttrs.set('href', safeHref)
      if (/^https?:\/\//i.test(safeHref)) {
        sanitizedAttrs.set('target', '_blank')
        sanitizedAttrs.set('rel', 'noopener noreferrer')
      }
      continue
    }

    if (name === 'src' || name === 'poster') {
      const normalized = normalizeMediaUrl(value, imageUrls)
      if (!normalized) {
        if (tag === 'iframe') return escapeHtml(fullTag)
        continue
      }
      sanitizedAttrs.set(name, normalized.url)
      if (tag === 'iframe' && name === 'src') {
        sanitizedAttrs.set(
          'sandbox',
          normalized.external
            ? 'allow-scripts allow-same-origin allow-popups allow-presentation'
            : 'allow-scripts allow-popups allow-presentation'
        )
      }
      continue
    }

    if (BOOLEAN_ATTRS.has(name)) {
      if (value.toLowerCase() !== 'false' && value !== '0') {
        sanitizedAttrs.set(name, '')
      }
      continue
    }

    sanitizedAttrs.set(name, value)
  }

  if (tag === 'iframe') {
    if (!sanitizedAttrs.has('src')) return escapeHtml(fullTag)
    if (!sanitizedAttrs.has('loading')) sanitizedAttrs.set('loading', 'lazy')
    if (!sanitizedAttrs.has('referrerpolicy'))
      sanitizedAttrs.set('referrerpolicy', 'strict-origin-when-cross-origin')
    if (!sanitizedAttrs.has('allowfullscreen')) sanitizedAttrs.set('allowfullscreen', '')
    if (!sanitizedAttrs.has('frameborder')) sanitizedAttrs.set('frameborder', '0')
  } else if (tag === 'img') {
    if (!sanitizedAttrs.has('src')) return escapeHtml(fullTag)
  } else if (tag === 'a' && sanitizedAttrs.get('target') === '_blank') {
    sanitizedAttrs.set('rel', 'noopener noreferrer')
  }

  const attrParts: string[] = []
  for (const [k, v] of sanitizedAttrs.entries()) {
    if (BOOLEAN_ATTRS.has(k) && v === '') {
      attrParts.push(k)
    } else {
      attrParts.push(`${k}="${escapeHtml(v)}"`)
    }
  }

  const attrOut = attrParts.length ? ' ' + attrParts.join(' ') : ''
  return `<${tag}${attrOut}>`
}

export function sanitizeMarkdownHtml(
  rawHtml: string,
  imageUrls?: Record<string, string>
): string {
  if (!rawHtml) return ''
  // Remove HTML comments first
  const withoutComments = rawHtml.replace(/<!--[\s\S]*?-->/g, '')
  // Sanitize or escape every HTML tag
  return withoutComments.replace(
    /<\/?[a-zA-Z][a-zA-Z0-9:-]*(?:\s+[^<>]*?)?\s*\/?>/g,
    tag => sanitizeSingleTag(tag, imageUrls)
  )
}

export function renderMediaEmbed(
  rawUrl: string,
  title = '',
  imageUrls?: Record<string, string>
): string | null {
  const trimmed = rawUrl.trim()
  if (!trimmed) return null
  const safeTitle = escapeHtml(title || '嵌入媒体')

  // 1. Bilibili BV / av / player URL
  const biliBvMatch =
    /(?:bilibili\.com\/video\/|player\.bilibili\.com\/player\.html\?[^"]*?bvid=|^)(BV[a-zA-Z0-9]{10})(?:[^\s"]*?[?&]p=(\d+))?/i.exec(
      trimmed
    )
  if (biliBvMatch) {
    const bvid = biliBvMatch[1]!
    const page = biliBvMatch[2] || '1'
    const embedUrl = `https://player.bilibili.com/player.html?isOutside=true&bvid=${encodeURIComponent(bvid)}&p=${encodeURIComponent(page)}&autoplay=0`
    return `<div class="markdown-media"><iframe src="${embedUrl}" title="${safeTitle}" sandbox="allow-scripts allow-same-origin allow-popups allow-presentation" referrerpolicy="strict-origin-when-cross-origin" loading="lazy" frameborder="0" scrolling="no" allowfullscreen></iframe></div>`
  }

  // 2. YouTube watch / youtu.be / embed URL
  const ytMatch =
    /(?:youtube(?:-nocookie)?\.com\/(?:watch\?v=|embed\/|shorts\/)|youtu\.be\/)([a-zA-Z0-9_-]{11})/i.exec(
      trimmed
    )
  if (ytMatch) {
    const videoId = ytMatch[1]!
    const embedUrl = `https://www.youtube-nocookie.com/embed/${encodeURIComponent(videoId)}`
    return `<div class="markdown-media"><iframe src="${embedUrl}" title="${safeTitle}" sandbox="allow-scripts allow-same-origin allow-popups allow-presentation" referrerpolicy="strict-origin-when-cross-origin" loading="lazy" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe></div>`
  }

  // 3. Direct video / audio or generic HTTPS embed URL
  const normalized = normalizeMediaUrl(trimmed, imageUrls)
  if (!normalized) return null

  if (/\.(mp4|webm|ogg|mov|m4v)(?:[?#].*)?$/i.test(normalized.url) || !normalized.external) {
    return `<div class="markdown-media"><video controls preload="metadata" playsinline src="${escapeHtml(normalized.url)}" title="${safeTitle}"></video></div>`
  }

  if (/\.(mp3|wav|flac|m4a)(?:[?#].*)?$/i.test(normalized.url)) {
    return `<div class="markdown-media"><audio controls preload="metadata" src="${escapeHtml(normalized.url)}" title="${safeTitle}"></audio></div>`
  }

  return `<div class="markdown-media"><iframe src="${escapeHtml(normalized.url)}" title="${safeTitle}" sandbox="allow-scripts allow-same-origin allow-popups allow-presentation" referrerpolicy="strict-origin-when-cross-origin" loading="lazy" frameborder="0" allowfullscreen></iframe></div>`
}

export function renderSandboxedHtmlDemo(source: string, info: string): string {
  const heightMatch = /\bheight=(\d{2,4})\b/i.exec(info)
  const height = heightMatch ? Math.min(1200, Math.max(120, Number(heightMatch[1]))) : 280
  const hasFullDoc = /<!doctype\s+html|<html[\s>]/i.test(source)
  const doc = hasFullDoc
    ? source
    : `<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>:root{color-scheme:dark light}body{margin:0;padding:16px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;line-height:1.6}</style></head><body>${source}</body></html>`

  return `<div class="code-block html-demo-block"><div class="code-top"><div class="code-title"><span class="code-dots" aria-hidden="true"><i></i><i></i><i></i></span><span class="code-lang">HTML DEMO</span></div></div><iframe class="html-demo-frame" title="动态 HTML 预览" sandbox="allow-scripts allow-forms allow-popups" referrerpolicy="no-referrer" loading="lazy" style="height:${height}px" srcdoc="${escapeHtml(doc)}"></iframe></div>\n`
}
