import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, existsSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = fileURLToPath(new URL('.', import.meta.url))
const rootDir = resolve(__dirname, '..')

test('landing component and layout files exist', () => {
  const requiredFiles = [
    'components/landing/TerminalBackdrop.vue',
    'components/landing/HeroSection.vue',
    'components/landing/FeaturedModule.vue',
    'components/landing/ModuleRail.vue',
    'layouts/landing.vue',
    'pages/index.vue'
  ]

  for (const file of requiredFiles) {
    const fullPath = resolve(rootDir, file)
    assert.ok(existsSync(fullPath), `File should exist: ${file}`)
    const content = readFileSync(fullPath, 'utf-8')
    assert.ok(content.length > 0, `File should not be empty: ${file}`)
    assert.ok(content.includes('<template>'), `File should be a Vue SFC: ${file}`)
  }
})

test('TerminalBackdrop.vue consumes useTerminalPlayer and defines terminal-screen, feed, and cursor', () => {
  const content = readFileSync(resolve(rootDir, 'components/landing/TerminalBackdrop.vue'), 'utf-8')

  // Consumes composable
  assert.ok(content.includes('useTerminalPlayer'))
  assert.ok(content.includes('branch'))
  assert.ok(content.includes('typedCommand'))
  assert.ok(content.includes('showCursor'))
  assert.ok(content.includes('feedLines'))
  assert.ok(content.includes('isClearing'))

  // Structure
  assert.ok(content.includes('terminal-backdrop'))
  assert.ok(content.includes('aria-hidden="true"'))
  assert.ok(content.includes('terminal-screen'))
  assert.ok(content.includes('terminal-feed'))
  assert.ok(content.includes('is-clearing'))
  assert.ok(content.includes('// {{ branch }}'))
  assert.ok(content.includes('$ {{ typedCommand }}'))
  assert.ok(content.includes('v-if="showCursor"'))
  assert.ok(content.includes('class="cursor"'))
  assert.ok(content.includes('v-for="(row, idx) in feedLines"'))
  assert.ok(content.includes(':class="row.type"'))
  assert.ok(content.includes('{{ row.text }}'))

  // Styles
  assert.ok(content.includes('radial-gradient(ellipse at 18% 50%, #a0674917, transparent 55%)'))
  assert.ok(content.includes('SFMono-Regular'))
  assert.ok(content.includes('Consolas'))
  assert.ok(content.includes('monospace'))
  assert.ok(content.includes('animation: blink'))
})

test('landing uses public module data and direct browsing links', () => {
  const page = readFileSync(resolve(rootDir, 'pages/index.vue'), 'utf-8')
  const hero = readFileSync(resolve(rootDir, 'components/landing/HeroSection.vue'), 'utf-8')
  const header = readFileSync(resolve(rootDir, 'layouts/landing.vue'), 'utf-8')
  assert.ok(page.includes('await useFetch'))
  assert.ok(page.includes("'/api/landing'"))
  assert.ok(page.includes('catalog-empty'))
  for (const file of ['FeaturedModule.vue', 'ModuleRail.vue']) {
    const source = readFileSync(resolve(rootDir, 'components/landing', file), 'utf-8')
    assert.ok(source.includes('/learn/modules/${module.slug}'))
    assert.ok(!source.includes('/auth'))
    assert.ok(!source.includes('次实践'))
  }
  assert.ok(hero.includes('to="/learn"'))
  assert.ok(header.includes('to="/learn"'))
})

test('database-backed landing is not frozen at build time', () => {
  const config = readFileSync(resolve(rootDir, 'nuxt.config.ts'), 'utf-8')
  assert.ok(config.includes("'/': { prerender: false }"))
  assert.ok(!config.includes("routes: ['/',"))
})
