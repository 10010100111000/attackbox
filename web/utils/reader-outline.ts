import type { MarkdownHeading } from './markdown'

export interface HeadingNode extends MarkdownHeading {
  children: HeadingNode[]
}

/** Markdown headings remain in document order; skipped levels never create fake headings. */
export function buildHeadingTree(headings: readonly MarkdownHeading[]): HeadingNode[] {
  const roots: HeadingNode[] = []
  const ancestors: HeadingNode[] = []
  for (const heading of headings) {
    const node: HeadingNode = { ...heading, children: [] }
    while (ancestors.length && ancestors[ancestors.length - 1]!.level >= node.level) ancestors.pop()
    const parent = ancestors[ancestors.length - 1]
    if (parent) parent.children.push(node)
    else roots.push(node)
    ancestors.push(node)
  }
  return roots
}

export function headingPath(nodes: readonly HeadingNode[], id: string): string[] {
  for (const node of nodes) {
    if (node.id === id) return [node.id]
    const child = headingPath(node.children, id)
    if (child.length) return [node.id, ...child]
  }
  return []
}
