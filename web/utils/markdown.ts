import { markdownHeadings } from './markdown-headings.ts'
import MarkdownIt from 'markdown-it'
import { callouts } from './markdown-callouts.ts'
import { markdownExtensions } from './markdown-extensions.ts'
import hljs from 'highlight.js/lib/common'

export interface MarkdownHeading {
  id: string
  text: string
  level: number
}

export interface MarkdownRenderResult {
  html: string
  headings: MarkdownHeading[]
}

export interface MarkdownOptions {
  copyCode?: boolean
}

interface MarkdownRenderEnv extends Record<PropertyKey, unknown> {
  imageUrls?: Record<string, string>
  copyCode?: boolean
  headings?: MarkdownHeading[]
}

export function createMarkdownRenderer(options: MarkdownOptions = {}) {
  const md = new MarkdownIt({
    html: false, // 严格转义任何原生 HTML，杜绝 XSS 注入
    linkify: true,
    breaks: false,
    typographer: true
  })

  md.use(callouts)
  md.use(markdownExtensions)

  // Scroll the table itself, keeping the surrounding document stationary.
  md.renderer.rules.table_open = () =>
    '<div class="markdown-table" tabindex="0" role="region" aria-label="表格"><table>\n'
  md.renderer.rules.table_close = () => '</table></div>\n'

  // The fence renderer returns the complete code card directly, avoiding
  // MarkdownIt's default extra <pre><code> wrapper and supporting per-render env options.
  md.renderer.rules.fence = (tokens, idx, _options, env?: MarkdownRenderEnv) => {
    const token = tokens[idx]!
    const str = token.content
    const lang = token.info.trim().split(/\s+/)[0] || ''
    const langName = lang.trim().toLowerCase()
    const displayLang = md.utils.escapeHtml(langName ? langName.toUpperCase() : 'CODE')
    let highlighted: string
    if (langName && hljs.getLanguage(langName)) {
      try {
        highlighted = hljs.highlight(str, { language: langName, ignoreIllegals: true }).value
      } catch {
        highlighted = md.utils.escapeHtml(str)
      }
    } else {
      highlighted = md.utils.escapeHtml(str)
    }
    const showCopy =
      env?.copyCode !== undefined ? env.copyCode !== false : options.copyCode !== false
    const copyButton = showCopy
      ? '<button type="button" class="copy-code-btn">复制代码</button>'
      : ''
    return `<div class="code-block"><div class="code-top"><span>${displayLang}</span>${copyButton}</div><pre><code class="hljs ${langName ? 'language-' + md.utils.escapeHtml(langName) : ''}">${highlighted}</code></pre></div>\n`
  }

  // 严格链接协议校验：仅允许 http, https, mailto 或相对链接锚点
  const defaultRenderLink =
    md.renderer.rules.link_open ||
    function (tokens, idx, renderOptions, _env, self) {
      return self.renderToken(tokens, idx, renderOptions)
    }

  md.renderer.rules.link_open = function (tokens, idx, renderOptions, env, self) {
    const hrefIndex = tokens[idx].attrIndex('href')
    if (hrefIndex >= 0) {
      const href = String(tokens[idx].attrs![hrefIndex][1])
      const isSafe =
        /^https?:\/\//i.test(href) ||
        /^mailto:/i.test(href) ||
        /^#/i.test(href) ||
        /^\//i.test(href)
      if (!isSafe) {
        tokens[idx].attrs![hrefIndex][1] = '#unsafe-link-blocked'
      }
      // 外部链接追加 target="_blank" 和 rel="noopener noreferrer"
      if (/^https?:\/\//i.test(href)) {
        tokens[idx].attrSet('target', '_blank')
        tokens[idx].attrSet('rel', 'noopener noreferrer')
      }
    }
    return defaultRenderLink(tokens, idx, renderOptions, env, self)
  }

  const defaultRenderImage =
    md.renderer.rules.image ||
    function (tokens, idx, renderOptions, _env, self) {
      return self.renderToken(tokens, idx, renderOptions)
    }

  md.renderer.rules.image = (
    tokens,
    index,
    renderOptions,
    env: MarkdownRenderEnv | undefined,
    self
  ) => {
    const token = tokens[index]!
    const href = String(token.attrGet('src') || '')
    const imageUrls = env?.imageUrls
    if (imageUrls && Object.hasOwn(imageUrls, href)) {
      token.attrSet('src', imageUrls[href]!)
    }
    return defaultRenderImage(tokens, index, renderOptions, env, self)
  }

  md.core.ruler.push('heading_ids', state => {
    const headings = markdownHeadings(state.tokens)
    const env = state.env as MarkdownRenderEnv
    env.headings?.push(...headings)
  })

  return md
}

const defaultMarkdownRenderer = createMarkdownRenderer()

export function renderMarkdown(
  content: string,
  imageUrls: Record<string, string> = {},
  options: MarkdownOptions = {}
): MarkdownRenderResult {
  if (!content) {
    return { html: '', headings: [] }
  }

  const headings: MarkdownHeading[] = []
  const env: MarkdownRenderEnv = {
    imageUrls,
    copyCode: options.copyCode,
    headings
  }

  const html = defaultMarkdownRenderer.render(content, env)

  return { html, headings }
}
