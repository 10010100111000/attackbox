import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import javascript from 'highlight.js/lib/languages/javascript'
import typescript from 'highlight.js/lib/languages/typescript'
import json from 'highlight.js/lib/languages/json'
import markdown from 'highlight.js/lib/languages/markdown'
import python from 'highlight.js/lib/languages/python'
import sql from 'highlight.js/lib/languages/sql'
import yaml from 'highlight.js/lib/languages/yaml'
import xml from 'highlight.js/lib/languages/xml'
import c from 'highlight.js/lib/languages/c'
import cpp from 'highlight.js/lib/languages/cpp'
import ini from 'highlight.js/lib/languages/ini'

hljs.registerLanguage('bash', bash)
hljs.registerLanguage('sh', bash)
hljs.registerLanguage('shell', bash)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('js', javascript)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('ts', typescript)
hljs.registerLanguage('json', json)
hljs.registerLanguage('markdown', markdown)
hljs.registerLanguage('md', markdown)
hljs.registerLanguage('python', python)
hljs.registerLanguage('py', python)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('yaml', yaml)
hljs.registerLanguage('yml', yaml)
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('html', xml)
hljs.registerLanguage('c', c)
hljs.registerLanguage('cpp', cpp)
hljs.registerLanguage('ini', ini)

export interface MarkdownHeading {
  id: string
  text: string
  level: number
}

export interface MarkdownRenderResult {
  html: string
  headings: MarkdownHeading[]
}

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\u4e00-\u9fa5\s-]/g, '')
    .replace(/[\s_-]+/g, '-')
    .replace(/^-+|-+$/g, '') || 'section'
}

export function createMarkdownRenderer() {
  const md = new MarkdownIt({
    html: false, // 严格转义任何原生 HTML，杜绝 XSS 注入
    linkify: true,
    breaks: false,
    typographer: true,
    highlight: (str: string, lang: string): string => {
      const langName = (lang || '').trim().toLowerCase()
      const displayLang = md.utils.escapeHtml(langName ? langName.toUpperCase() : 'CODE')
      let highlighted = ''
      if (langName && hljs.getLanguage(langName)) {
        try {
          highlighted = hljs.highlight(str, { language: langName, ignoreIllegals: true }).value
        } catch {
          highlighted = md.utils.escapeHtml(str)
        }
      } else {
        highlighted = md.utils.escapeHtml(str)
      }
      return `<div class="code-block"><div class="code-top"><span>${displayLang}</span><button type="button" class="copy-code-btn" data-code="${encodeURIComponent(str)}">复制代码</button></div><pre><code class="hljs ${langName ? 'language-' + md.utils.escapeHtml(langName) : ''}">${highlighted}</code></pre></div>`
    },
  })

  // The highlighter already returns the complete code card. Avoid MarkdownIt's
  // default extra <pre><code> wrapper, which indents and narrows the card.
  md.renderer.rules.fence = (tokens, idx, options) => {
    const token = tokens[idx]!
    const language = token.info.trim().split(/\s+/)[0] || ''
    return options.highlight ? `${options.highlight(token.content, language, '')}\n` : ''
  }

  // 严格链接协议校验：仅允许 http, https, mailto 或相对链接锚点
  const defaultRenderLink =
    md.renderer.rules.link_open ||
    function (tokens, idx, options, _env, self) {
      return self.renderToken(tokens, idx, options)
    }

  md.renderer.rules.link_open = function (tokens, idx, options, env, self) {
    const hrefIndex = tokens[idx].attrIndex('href')
    if (hrefIndex >= 0) {
      const href = String(tokens[idx].attrs![hrefIndex][1])
      const isSafe = /^https?:\/\//i.test(href) || /^mailto:/i.test(href) || /^#/i.test(href) || /^\//i.test(href)
      if (!isSafe) {
        tokens[idx].attrs![hrefIndex][1] = '#unsafe-link-blocked'
      }
      // 外部链接追加 target="_blank" 和 rel="noopener noreferrer"
      if (/^https?:\/\//i.test(href)) {
        tokens[idx].attrSet('target', '_blank')
        tokens[idx].attrSet('rel', 'noopener noreferrer')
      }
    }
    return defaultRenderLink(tokens, idx, options, env, self)
  }

  return md
}

export function renderMarkdown(content: string): MarkdownRenderResult {
  if (!content) {
    return { html: '', headings: [] }
  }

  const md = createMarkdownRenderer()
  const headings: MarkdownHeading[] = []
  const usedSlugs = new Map<string, number>()

  // 提取 headings 并附加 id
  const defaultHeadingOpen =
    md.renderer.rules.heading_open ||
    function (tokens, idx, options, _env, self) {
      return self.renderToken(tokens, idx, options)
    }

  md.renderer.rules.heading_open = function (tokens, idx, options, env, self) {
    const token = tokens[idx]
    const level = parseInt(token.tag.slice(1), 10) || 2
    const nextToken = tokens[idx + 1]
    const titleText = nextToken && nextToken.type === 'inline' ? nextToken.content : ''

    let baseSlug = slugify(titleText)
    const count = usedSlugs.get(baseSlug) || 0
    usedSlugs.set(baseSlug, count + 1)
    const id = count === 0 ? baseSlug : `${baseSlug}-${count}`

    headings.push({ id, text: titleText, level })
    token.attrSet('id', id)

    return defaultHeadingOpen(tokens, idx, options, env, self)
  }

  let html = md.render(content)

  // 处理 GitHub 风格 Callout 提示框：
  // <blockquote><p>[!NOTE] ... </p></blockquote> -> <div class="callout callout-note">...</div>
  html = html.replace(
    /<blockquote>\s*<p>\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*([\s\S]*?)<\/blockquote>/gi,
    (_match, type, body) => {
      const calloutType = String(type).toLowerCase()
      const titleMap: Record<string, string> = {
        note: '本章学习目标',
        tip: '停下来，想一想',
        important: '重要提示',
        warning: '安全警告',
        caution: '风险警示',
      }
      const title = titleMap[calloutType] || '提示'
      return `<div class="callout callout-${calloutType}"><strong>${title}</strong><p>${body}</div>`
    }
  )

  return { html, headings }
}
