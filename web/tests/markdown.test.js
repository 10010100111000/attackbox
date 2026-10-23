// web/tests/markdown.test.js
import test from 'node:test';
import assert from 'node:assert/strict';
import { renderMarkdown } from '../utils/markdown.ts';

test('renderMarkdown converts headers, bold, and list with heading extraction', () => {
  const md = `# Linux 基础入门\n\n掌握 **常用命令** 与权限：\n\n- pwd\n- ls\n- cd\n\n## 认识终端与 Shell\n\nShell 负责解释命令。`;
  const result = renderMarkdown(md);

  assert.ok(result.html.includes('<h1 id="linux-基础入门">Linux 基础入门</h1>'));
  assert.ok(result.html.includes('<strong>常用命令</strong>'));
  assert.ok(result.html.includes('<li>pwd</li>'));
  assert.equal(result.headings.length, 2);
  assert.equal(result.headings[0].text, 'Linux 基础入门');
  assert.equal(result.headings[0].level, 1);
  assert.equal(result.headings[1].text, '认识终端与 Shell');
  assert.equal(result.headings[1].level, 2);
});

test('renderMarkdown strictly prevents XSS by escaping raw html tags', () => {
  const attack = `<script>alert(1)</script><img src="x" onerror="alert(2)"><a href="javascript:alert(3)">click</a>`;
  const result = renderMarkdown(attack);

  assert.ok(!result.html.includes('<script>'), 'script tag must be escaped');
  assert.ok(result.html.includes('&lt;script&gt;alert(1)&lt;/script&gt;'));
  assert.ok(!result.html.includes('onerror="alert(2)"'), 'onerror must not remain active tag');
  assert.ok(!result.html.includes('href="javascript:alert(3)"'), 'javascript: scheme must be sanitized');
});

test('renderMarkdown generates code blocks with highlight and copy data attribute', () => {
  const codeMd = "```bash\npwd\necho 'hello world'\n```";
  const result = renderMarkdown(codeMd);

  assert.ok(result.html.includes('class="code-block"'), 'Should wrap in code-block container');
  assert.ok(result.html.includes('class="copy-code-btn"'), 'Should include copy button');
  assert.ok(result.html.includes('BASH'), 'Should display language');
  assert.ok(result.html.includes('pwd'));
  assert.equal((result.html.match(/<pre(?:\s|>)/g) || []).length, 1, 'code card must have a single pre element');
  assert.ok(result.html.startsWith('<div class="code-block">'), 'code card must not be nested inside an extra pre');
});

test('renderMarkdown renders callout containers for blockquotes with type tags', () => {
  const noteMd = `> [!NOTE]\n> 理解核心概念，读懂示例命令。\n\n> [!TIP]\n> 停下来思考命令的作用。`;
  const result = renderMarkdown(noteMd);

  assert.ok(result.html.includes('class="callout callout-note"'), 'Should render note callout');
  assert.ok(result.html.includes('本章学习目标'));
  assert.ok(result.html.includes('class="callout callout-tip"'), 'Should render tip callout');
  assert.ok(result.html.includes('停下来，想一想'));
});

test('fenced language labels escape HTML and attribute delimiters', () => {
  const result = renderMarkdown('```unknown"onclick="alert(1)\nhello\n```');
  assert.ok(!result.html.includes('unknown"onclick='));
  assert.ok(result.html.includes('&quot;'));
});
