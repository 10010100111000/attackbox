import test from 'node:test'
import assert from 'node:assert/strict'
import { defaultLearnHome, parseLearnHome } from '../shared/learn-home.ts'
test('homepage validates links, identity, content and timing before persistence', () => {
  const config = structuredClone(defaultLearnHome)
  assert.deepEqual(parseLearnHome(config), config)
  for (const href of [
    'https://example.com',
    '//evil.test',
    '/\\evil',
    'javascript:alert(1)',
    '/path\nnext'
  ]) {
    const bad = structuredClone(config)
    bad.slides[0].href = href
    assert.throws(() => parseLearnHome(bad))
  }
  for (const patch of [
    { intervalSeconds: 0 },
    { intervalSeconds: 31 },
    { lockVersion: -1 },
    { slides: [config.slides[0], config.slides[0]] },
    { slides: Array(11).fill(config.slides[0]) }
  ])
    assert.throws(() => parseLearnHome({ ...config, ...patch }))
  const missing = structuredClone(config)
  missing.slides[0].title = ' '
  assert.throws(() => parseLearnHome(missing))
  assert.deepEqual(parseLearnHome({ ...config, enabled: false, slides: [] }).slides, [])
})
