import MarkdownIt from 'markdown-it'
import { callouts } from './markdown-callouts.ts'
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
  return (
    text
      .toLowerCase()
      .trim()
      .replace(/[^\p{L}\p{N}\s_-]/gu, '')
      .replace(/[\s_-]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'section'
  )
}

export interface MarkdownOptions {
  copyCode?: boolean
}

interface MarkdownRenderEnv extends Record<PropertyKey, unknown> {
  imageUrls?: Record<string, string>
  copyCode?: boolean
  headings?: MarkdownHeading[]
  usedSlugs?: Set<string>
}

export function createMarkdownRenderer(options: MarkdownOptions = {}) {
  const md = new MarkdownIt({
    html: false, // 严格转义任何原生 HTML，杜绝 XSS 注入
    linkify: true,
    breaks: false,
    typographer: true
  })

  md.use(callouts)

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

  // 提取 headings 并附加 id
  const defaultHeadingOpen =
    md.renderer.rules.heading_open ||
    function (tokens, idx, renderOptions, _env, self) {
      return self.renderToken(tokens, idx, renderOptions)
    }

  md.renderer.rules.heading_open = function (
    tokens,
    idx,
    renderOptions,
    env: MarkdownRenderEnv | undefined,
    self
  ) {
    const token = tokens[idx]!
    const level = parseInt(token.tag.slice(1), 10) || 2
    const nextToken = tokens[idx + 1]
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

    const baseSlug = slugify(titleText)
    let id = baseSlug
    let suffix = 1
    const usedSlugs = env?.usedSlugs
    if (usedSlugs) {
      while (usedSlugs.has(id)) id = `${baseSlug}-${suffix++}`
      usedSlugs.add(id)
    }

    if (env?.headings) {
      env.headings.push({ id, text: titleText, level })
    }
    token.attrSet('id', id)

    return defaultHeadingOpen(tokens, idx, renderOptions, env, self)
  }

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
  const usedSlugs = new Set<string>()
  const env: MarkdownRenderEnv = {
    imageUrls,
    copyCode: options.copyCode,
    headings,
    usedSlugs
  }

  const html = defaultMarkdownRenderer.render(content, env)

  return { html, headings }
}
