import test from 'node:test'
import assert from 'node:assert/strict'
import { referenceTemplate } from '../server/content/reference-template.ts'
import { parseReferenceMarkdown } from '../server/content/parse-reference.ts'
test('reference documents use filename titles, optional summary and ordinary markdown', () => {
  const doc=parseReferenceMarkdown('books/网络基础.md','---\nsummary: 网络学习笔记\n---\n\n## 协议\n\n正文 ![图](images/a.png)\n\n```bash\nwhoami\n```')
  assert.equal(doc.title,'网络基础');assert.equal(doc.summary,'网络学习笔记');assert.ok(doc.bodyMarkdown.startsWith('## 协议'))
  assert.equal(doc.images[0].href,'images/a.png')
  assert.equal(parseReferenceMarkdown('普通资料.md','# 标题\n\n无需配置头').summary,'')
  assert.equal(parseReferenceMarkdown('empty-header.md','---\n---\n正文').bodyMarkdown,'正文')
})
test('reference parsing rejects malformed metadata, paths, oversized documents and empty content', () => {
  for(const [file,text] of [['../a.md','text'],['a.txt','text'],['a.md',''],['a.md','---\nsummary: a\nsummary: b\n---\ntext'],['a.md','---\naccess: authenticated\n---\ntext'],['a.md','---\nsummary: [one]\n---\ntext'],['a.md','---\nx: &x [*x]\n---\ntext'],['a.md','a'.repeat(2*1024*1024+1)]])assert.throws(()=>parseReferenceMarkdown(file,text))
})

test('downloadable reference template follows the reference format',()=>{const doc=parseReferenceMarkdown('参考资料示例.md',referenceTemplate);assert.ok(doc.bodyMarkdown.includes('## 阅读笔记'));assert.ok(doc.summary)})
