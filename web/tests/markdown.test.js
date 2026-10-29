// web/tests/markdown.test.js
import test from 'node:test'
import assert from 'node:assert/strict'
import { renderMarkdown, createMarkdownRenderer } from '../utils/markdown.ts'

test('renderMarkdown converts headers, bold, and list with heading extraction', () => {
  const md = `# Linux 基础入门\n\n掌握 **常用命令** 与权限：\n\n- pwd\n- ls\n- cd\n\n## 认识终端与 Shell\n\nShell 负责解释命令。`
  const result = renderMarkdown(md)

  assert.ok(result.html.includes('<h1 id="linux-基础入门">Linux 基础入门</h1>'))
  assert.ok(result.html.includes('<strong>常用命令</strong>'))
  assert.ok(result.html.includes('<li>pwd</li>'))
  assert.equal(result.headings.length, 2)
  assert.equal(result.headings[0].text, 'Linux 基础入门')
  assert.equal(result.headings[0].level, 1)
  assert.equal(result.headings[1].text, '认识终端与 Shell')
  assert.equal(result.headings[1].level, 2)
})

test('renderMarkdown strictly prevents XSS by escaping raw html tags', () => {
  const attack = `<script>alert(1)</script><img src="x" onerror="alert(2)"><a href="javascript:alert(3)">click</a>`
  const result = renderMarkdown(attack)

  assert.ok(!result.html.includes('<script>'), 'script tag must be escaped')
  assert.ok(result.html.includes('&lt;script&gt;alert(1)&lt;/script&gt;'))
  assert.ok(!result.html.includes('onerror="alert(2)"'), 'onerror must not remain active tag')
  assert.ok(
    !result.html.includes('href="javascript:alert(3)"'),
    'javascript: scheme must be sanitized'
  )
})

test('renderMarkdown generates code blocks with highlight and without redundant data-code payload', () => {
  const codeMd = "```bash\npwd\necho 'hello world'\n```"
  const result = renderMarkdown(codeMd)

  assert.ok(result.html.includes('class="code-block"'), 'Should wrap in code-block container')
  assert.ok(result.html.includes('class="copy-code-btn"'), 'Should include copy button')
  assert.ok(
    !result.html.includes('data-code='),
    'Should not duplicate raw code into data-code attribute'
  )
  assert.ok(result.html.includes('BASH'), 'Should display language')
  assert.ok(result.html.includes('pwd'))
  assert.equal(
    (result.html.match(/<pre(?:\s|>)/g) || []).length,
    1,
    'code card must have a single pre element'
  )
  assert.ok(
    result.html.startsWith('<div class="code-block">'),
    'code card must not be nested inside an extra pre'
  )
})

test('renderMarkdown renders callout containers for blockquotes with type tags', () => {
  const noteMd = `> [!NOTE]\n> 理解核心概念，读懂示例命令。\n\n> [!TIP]\n> 停下来思考命令的作用。`
  const result = renderMarkdown(noteMd)

  assert.ok(result.html.includes('class="callout callout-note"'), 'Should render note callout')
  assert.ok(result.html.includes('本章学习目标'))
  assert.ok(result.html.includes('class="callout callout-tip"'), 'Should render tip callout')
  assert.ok(result.html.includes('停下来，想一想'))
})

test('fenced language labels escape HTML and attribute delimiters', () => {
  const result = renderMarkdown('```unknown"onclick="alert(1)\nhello\n```')
  assert.ok(!result.html.includes('unknown"onclick='))
  assert.ok(result.html.includes('&quot;'))
})

test('callouts preserve multiple paragraphs, lists, fences and nested quotes', () => {
  const result = renderMarkdown(
    '> [!NOTE]\n>\n> First **paragraph**.\n>\n> Second paragraph.\n>\n> - one\n> - two\n>\n> ```bash\n> echo safe\n> ```\n>\n> > ordinary quote\n>\n> Last paragraph.\n\nOutside.'
  )
  assert.match(
    result.html,
    /^<div class="callout callout-note">\s*<strong>本章学习目标<\/strong>\s*<p>First/
  )
  assert.ok(result.html.includes('</p>\n<p>Second paragraph.</p>\n<ul>'))
  assert.ok(result.html.includes('</ul>\n<div class="code-block">'))
  assert.ok(result.html.includes('<blockquote>\n<p>ordinary quote</p>\n</blockquote>'))
  assert.ok(result.html.endsWith('<p>Last paragraph.</p>\n</div>\n<p>Outside.</p>\n'))
  assert.equal((result.html.match(/<p>/g) || []).length, (result.html.match(/<\/p>/g) || []).length)
})

test('nested and adjacent callouts keep their own closing tokens and headings', () => {
  const result = renderMarkdown(
    '> [!WARNING]\n> Outer\n>\n> > [!TIP]\n> > Inner\n>\n> ## Nested heading\n>\n> Outer end\n\n> [!CAUTION]\n> Next'
  )
  assert.ok(result.html.includes('callout-warning'))
  assert.ok(result.html.includes('callout-tip'))
  assert.ok(result.html.includes('<p>Inner</p>\n</div>\n<h2 id="nested-heading">'))
  assert.ok(result.html.includes('<p>Outer end</p>\n</div>\n<div class="callout callout-caution">'))
  assert.deepEqual(result.headings, [{ id: 'nested-heading', text: 'Nested heading', level: 2 }])
})

test('callout recognition is limited to a leading supported marker, not code or HTML', () => {
  const md = createMarkdownRenderer()
  for (const source of [
    '> [!UNKNOWN]\n> body',
    '> prefix [!NOTE]',
    '> `[!TIP]`',
    '```md\n> [!NOTE]\n```'
  ]) {
    assert.ok(!md.render(source).includes('class="callout '))
  }
  assert.ok(md.render('> [!note] Inline **body**').includes('<p>Inline <strong>body</strong></p>'))
  const empty = md.render('> [!IMPORTANT]')
  assert.ok(!empty.includes('<p>'))
  assert.ok(empty.includes('重要提示'))
  const attack = md.render('> [!NOTE]\n> <script>alert(1)</script>')
  assert.ok(!attack.includes('<script>'))
  assert.ok(attack.includes('&lt;script&gt;'))
})

test('heading anchors are unique across duplicates and suffix collisions, and labels use rendered text', () => {
  const result = renderMarkdown(
    '## **Foo** `bar`\n\n## Foo bar\n\n## Foo bar-1\n\n## Foo bar\n\n```md\n## not a heading\n```\n\n## 你好 世界\n\n### [Child](https://example.org)'
  )
  assert.deepEqual(
    result.headings.map(h => h.id),
    ['foo-bar', 'foo-bar-1', 'foo-bar-1-1', 'foo-bar-2', '你好-世界', 'child']
  )
  assert.equal(result.headings[0].text, 'Foo bar')
})
test('tutorial rendering omits copy controls without changing code text or default renderers', () => {
  const source = '```bash\necho practice\n```'
  const tutorial = renderMarkdown(source, {}, { copyCode: false }).html
  assert.ok(tutorial.includes('echo'))
  assert.ok(!tutorial.includes('copy-code-btn'))
  assert.ok(!tutorial.includes('data-code'))
  assert.ok(renderMarkdown(source).html.includes('copy-code-btn'))
})
