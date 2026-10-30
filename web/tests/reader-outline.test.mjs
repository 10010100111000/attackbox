import test from 'node:test'
import assert from 'node:assert/strict'
import { buildHeadingTree, headingPath } from '../utils/reader-outline.ts'

const heading = (id, level) => ({ id, text: id, level })
test('reader outline preserves document order, skipped levels and distinct duplicate-heading anchors', () => {
  const headings = [
    heading('orphan', 3),
    heading('part', 1),
    heading('setup', 2),
    heading('detail', 4),
    heading('setup-1', 2),
    heading('next-part', 1)
  ]
  const nodes = buildHeadingTree(headings)
  assert.deepEqual(
    nodes.map(node => node.id),
    ['orphan', 'part', 'next-part']
  )
  assert.deepEqual(
    nodes[1].children.map(node => node.id),
    ['setup', 'setup-1']
  )
  assert.deepEqual(
    nodes[1].children[0].children.map(node => node.id),
    ['detail']
  )
  assert.deepEqual(headingPath(nodes, 'detail'), ['part', 'setup', 'detail'])
  assert.deepEqual(headingPath(nodes, 'missing'), [])
  assert.equal(
    'children' in headings[0],
    false,
    'building an outline never mutates renderer output'
  )
  assert.deepEqual(buildHeadingTree([]), [])
})
