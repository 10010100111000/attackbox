import test from 'node:test'
import assert from 'node:assert/strict'
import { compileReference } from '../server/content/reference-compiler.ts'
import { renderMarkdown } from '../utils/markdown.ts'

test('reading parts retain global anchors, cross-part link definitions, safe rendering and Markdown block semantics', () => {
  const source =
    'Intro\n\n# First\n\n## Repeat\n\n[link][destination]\n\n> [!NOTE]\n> paragraph\n>\n> - nested list\n\n# Second\n\n## Repeat\n\n```bash\n# not a heading\nwhoami\n```\n\n![image](a.png)\n\n<script>bad()</script>\n\n[destination]: https://example.com/reference\n'
  const compiled = compileReference(source, { 'a.png': '/assets/one' })
  assert.equal(compiled.chunks.length, 2)
  assert.deepEqual(
    compiled.outline.map(({ id, text, level }) => ({ id, text, level })),
    renderMarkdown(source).headings
  )
  assert.equal(compiled.outline.find(h => h.id === 'repeat-1').chunk, 1)
  const first = compiled.chunks[0].html,
    second = compiled.chunks[1].html
  assert.ok(first.includes('Intro') && first.includes('https://example.com/reference'))
  assert.ok(first.includes('callout') && first.includes('<ul>'))
  assert.ok(!first.includes('whoami'))
  assert.ok(second.includes('id="repeat-1"') && second.includes('whoami'))
  assert.ok(second.includes('src="/assets/one"'))
  assert.ok(!second.includes('<script>'))
})
test('documents without headings split by block size without breaking lists or fences', () => {
  const paragraphs = Array.from(
    { length: 100 },
    (_, i) => `Paragraph ${i}: ${'text '.repeat(300)}`
  ).join('\n\n')
  const compiled = compileReference(paragraphs)
  assert.ok(compiled.chunks.length > 4)
  assert.equal(compiled.outline.length, 0)
  assert.ok(compiled.chunks[0].html.length < 30000)
  assert.ok(!compiled.chunks[0].html.includes('Paragraph 99:'))
  const code = '```bash\n' + 'echo a\n'.repeat(5000) + '```'
  const atomic = compileReference(code)
  assert.equal(atomic.chunks.length, 1)
  assert.equal((atomic.chunks[0].html.match(/class="code-block"/g) || []).length, 1)
})
