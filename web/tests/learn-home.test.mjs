import test from 'node:test'
import assert from 'node:assert/strict'
import { defaultLearnHome, parseLearnHome } from '../shared/learn-home.ts'

test('homepage accepts image campaigns, announcements and HTTPS destinations without decorative fields', () => {
  const config = structuredClone(defaultLearnHome)
  assert.deepEqual(parseLearnHome(config), config)
  for (const patch of [
    {
      href: 'https://example.com/campaign?from=learn',
      imageUrl: 'https://cdn.example.com/banner.webp?v=2'
    },
    { href: '/learn?category=web#modules', imageUrl: '/media/banner.webp' },
    { href: '', cta: '', imageUrl: '', label: '', description: '' }
  ]) {
    const slide = { ...config.slides[0], ...patch }
    assert.deepEqual(parseLearnHome({ ...config, slides: [slide] }).slides[0], slide)
  }
  const extra = { ...config.slides[0], command: '$ old-decoration', caption: 'Old art' }
  assert.deepEqual(parseLearnHome({ ...config, slides: [extra] }).slides[0], config.slides[0])
})

test('homepage rejects unsafe links and invalid identity, content or timing before persistence', () => {
  const config = structuredClone(defaultLearnHome)
  for (const field of ['href', 'imageUrl']) {
    for (const url of [
      'http://example.com',
      '//evil.test',
      '/\\evil',
      'javascript:alert(1)',
      '/path\nnext',
      'data:image/svg+xml,test',
      'https://user:pass@example.com/banner',
      'https:example.com'
    ]) {
      const bad = structuredClone(config)
      bad.slides[0][field] = url
      assert.throws(() => parseLearnHome(bad), `${field}: ${url}`)
    }
  }
  for (const patch of [
    { intervalSeconds: 0 },
    { intervalSeconds: 31 },
    { lockVersion: -1 },
    { slides: [config.slides[0], config.slides[0]] },
    { slides: Array(11).fill(config.slides[0]) },
    { slides: [{ ...config.slides[0], title: ' ' }] },
    { slides: [{ ...config.slides[0], imageUrl: 123 }] }
  ])
    assert.throws(() => parseLearnHome({ ...config, ...patch }))
  assert.deepEqual(parseLearnHome({ ...config, enabled: false, slides: [] }).slides, [])
})
