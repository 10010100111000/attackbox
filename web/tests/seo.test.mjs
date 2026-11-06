import test from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  DEFAULT_SEO_KEYWORDS,
  DEFAULT_SITE_ORIGIN,
  buildKeywords,
  buildVerificationMeta,
  canonicalUrl,
  difficultySeoLabel,
  escapeXml,
  extractSeoExcerpt,
  normalizeSiteOrigin,
  serializeJsonLd,
  stripMarkdownToText
} from '../shared/seo.ts'

const __dirname = fileURLToPath(new URL('.', import.meta.url))
const rootDir = resolve(__dirname, '..')

test('normalizeSiteOrigin and canonicalUrl normalize origins and paths safely', () => {
  assert.equal(
    normalizeSiteOrigin('https://attackbox.cn/path', 'http://localhost:3000'),
    'https://attackbox.cn'
  )
  assert.equal(
    normalizeSiteOrigin('', 'https://learn.attackbox.cn:8443/learn'),
    'https://learn.attackbox.cn:8443'
  )
  assert.equal(normalizeSiteOrigin('invalid-url', ''), DEFAULT_SITE_ORIGIN)

  assert.equal(canonicalUrl('https://attackbox.cn', '/'), 'https://attackbox.cn/')
  assert.equal(
    canonicalUrl('https://attackbox.cn', '/learn/modules/linux-basics/?tab=1#top'),
    'https://attackbox.cn/learn/modules/linux-basics'
  )
})

test('extractSeoExcerpt strips markdown and truncates at Chinese punctuation', () => {
  const sample = [
    '---',
    'title: Linux 权限提升',
    '---',
    '# 第一章：SUID 原理与利用',
    '',
    '在本章中，我们将深入理解 `SUID` 权限位的工作机制，并通过 **实战终端** 查找可利用的二进制文件。',
    '```bash',
    'find / -perm -4000 2>/dev/null',
    '```',
    '随后我们将结合靶场环境完成提权验证，掌握常见防御加固方案。'
  ].join('\n')

  const plain = stripMarkdownToText(sample)
  assert.ok(!plain.includes('```'))
  assert.ok(!plain.includes('find / -perm'))
  assert.ok(plain.includes('第一章：SUID 原理与利用'))

  const excerpt = extractSeoExcerpt(sample, 65)
  assert.ok(excerpt.length <= 66)
  assert.ok(excerpt.includes('SUID'))
})

test('buildKeywords deduplicates Chinese and English delimiters and caps keyword count', () => {
  const keywords = buildKeywords(
    'AttackBox, 网络安全学习，渗透测试',
    ['Web安全实战、Linux安全入门', '网络安全学习'],
    difficultySeoLabel('beginner'),
    null,
    undefined
  )
  assert.equal(
    keywords,
    'AttackBox,网络安全学习,渗透测试,Web安全实战,Linux安全入门,基础入门'
  )
})

test('escapeXml and serializeJsonLd neutralize script/tag injection characters', () => {
  const unsafeTag = '\x3c/script\x3e\x3cscript\x3ealert("xss")\x3c/script\x3e'
  assert.equal(
    escapeXml('https://attackbox.cn/?a=1&b=2'),
    'https://attackbox.cn/?a=1&amp;b=2'
  )
  const json = serializeJsonLd({
    '@context': 'https://schema.org',
    name: unsafeTag
  })
  assert.ok(!json.includes('\x3c/script\x3e'))
  assert.ok(json.includes('\\u003c/script\\u003e'))
})

test('SEO server routes and Chinese meta configurations are wired in Nuxt', () => {
  for (const file of [
    'shared/seo.ts',
    'server/routes/robots.txt.get.ts',
    'server/routes/sitemap.xml.get.ts'
  ]) {
    const full = resolve(rootDir, file)
    assert.ok(existsSync(full), `Expected SEO file to exist: ${file}`)
  }

  const nuxtConfig = readFileSync(resolve(rootDir, 'nuxt.config.ts'), 'utf-8')
  assert.ok(nuxtConfig.includes("lang: 'zh-CN'"))
  assert.ok(nuxtConfig.includes("property: 'og:locale', content: 'zh_CN'"))
  assert.ok(nuxtConfig.includes("name: 'applicable-device', content: 'pc,mobile'"))
  assert.ok(DEFAULT_SEO_KEYWORDS.includes('网络安全学习'))

  const robotsRoute = readFileSync(
    resolve(rootDir, 'server/routes/robots.txt.get.ts'),
    'utf-8'
  )
  assert.ok(robotsRoute.includes('Baiduspider'))
  assert.ok(robotsRoute.includes('Baiduspider-render'))
  assert.ok(robotsRoute.includes('Sogou web spider'))
  assert.ok(robotsRoute.includes('360Spider'))
  assert.ok(robotsRoute.includes('Bytespider'))
  assert.ok(robotsRoute.includes('ToutiaoSpider'))
  assert.ok(robotsRoute.includes('YisouSpider'))
  assert.ok(robotsRoute.includes('Sitemap:'))
  assert.ok(robotsRoute.includes('/sitemap.txt'))

  for (const extraRoute of [
    'server/routes/sitemap.txt.get.ts',
    'server/routes/rss.xml.get.ts',
    'server/services/seo-sitemap.ts',
    'plugins/seo-meta.server.ts'
  ]) {
    assert.ok(existsSync(resolve(rootDir, extraRoute)), `Expected ${extraRoute} to exist`)
  }

  const metas = buildVerificationMeta({
    baidu: ' code-baidu ',
    bing: 'code-bing',
    sogou: 'code-sogou',
    so360: 'code-360',
    shenma: 'code-sm',
    bytedance: 'code-tt'
  })
  assert.equal(metas.length, 6)
  assert.deepEqual(metas[0], { name: 'baidu-site-verification', content: 'code-baidu' })
})
