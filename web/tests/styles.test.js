import test from 'node:test'
import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { resolve, relative } from 'node:path'
import { parse, compileStyle } from 'vue/compiler-sfc'

const root = resolve(import.meta.dirname, '..')
function files(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap(item => {
    const path = resolve(directory, item.name)
    return item.isDirectory() ? files(path) : /\.(css|vue)$/.test(item.name) ? [path] : []
  })
}

test('style owners do not repeat selectors inside the same cascade context', () => {
  const repeated = []
  for (const file of ['assets/css', 'components', 'layouts', 'pages'].flatMap(directory =>
    files(resolve(root, directory))
  )) {
    const source = readFileSync(file, 'utf8')
    const blocks = file.endsWith('.css')
      ? [source]
      : parse(source)
          .descriptor.styles.filter(style => !style.src)
          .map(style => style.content)
    for (const source of blocks) {
      const result = compileStyle({ source, filename: file, id: 'style-audit' })
      assert.deepEqual(result.errors, [], relative(root, file))
      const seen = new Set()
      result.rawResult.root.walkRules(rule => {
        const context = []
        for (let owner = rule.parent; owner && owner.type !== 'root'; owner = owner.parent) {
          context.unshift(owner.type === 'rule' ? owner.selector : `@${owner.name} ${owner.params}`)
        }
        const key = JSON.stringify([...context, rule.selector.trim()])
        if (seen.has(key)) repeated.push(`${relative(root, file)}: ${rule.selector}`)
        seen.add(key)
      })
    }
  }
  assert.deepEqual(
    repeated,
    [],
    'Merge changes into the owning rule instead of appending an override'
  )
})
