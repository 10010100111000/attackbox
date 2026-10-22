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
  assert.ok(content.includes('"SFMono-Regular", Consolas, monospace'))
  assert.ok(content.includes('@keyframes blink'))
})

test('FeaturedModule.vue renders 03 / CLOUD NATIVE card with cluster SVG and auth login link', () => {
  const content = readFileSync(resolve(rootDir, 'components/landing/FeaturedModule.vue'), 'utf-8')
  
  assert.ok(content.includes('to="/auth?mode=login"'))
  assert.ok(content.includes('featured-module'))
  assert.ok(content.includes('data-module="cloud"'))
  assert.ok(content.includes('data-level="advanced"'))
  assert.ok(content.includes('03 / CLOUD NATIVE'))
  assert.ok(content.includes('进阶'))
  assert.ok(content.includes('cloud-art'))
  assert.ok(content.includes('CLUSTER'))
  assert.ok(content.includes('云原生攻防'))
  assert.ok(content.includes('沿着身份、服务与集群寻找攻击路径。'))
  assert.ok(content.includes('10 章节 · 5 次实践'))
  assert.ok(content.includes('handleCardPointerMove'))
  assert.ok(content.includes('@pointermove="handleCardPointerMove"'))
})

test('ModuleRail.vue renders 5 modules with data-module, pointermove, code snippets, and accessibility', () => {
  const content = readFileSync(resolve(rootDir, 'components/landing/ModuleRail.vue'), 'utf-8')
  
  // Rail container & composable integration
  assert.ok(content.includes('useModuleRail(railRef)'))
  assert.ok(content.includes('ref="railRef"'))
  assert.ok(content.includes('tabindex="0"'))
  assert.ok(content.includes('role="region"'))
  assert.ok(content.includes('aria-label="更多学习模块，横向滚动浏览"'))

  // 5 Modules defined
  assert.ok(content.includes('data-module="linux"'))
  assert.ok(content.includes('data-module="web"'))
  assert.ok(content.includes('data-module="network"'))
  assert.ok(content.includes('data-module="reverse"'))
  assert.ok(content.includes('data-module="container"'))

  // 01 Linux
  assert.ok(content.includes('01 / SYSTEM'))
  assert.ok(content.includes('基础'))
  assert.ok(content.includes('/images/linux.jpg'))
  assert.ok(content.includes('Linux 与系统基础'))
  assert.ok(content.includes('8 章节 · 3 次实践'))
  assert.ok(content.includes('chmod u+x explore.sh'))

  // 02 Web
  assert.ok(content.includes('02 / WEB SECURITY'))
  assert.ok(content.includes('/images/web.jpg'))
  assert.ok(content.includes('Web 漏洞分析'))
  assert.ok(content.includes('12 章节 · 6 次实践'))
  assert.ok(content.includes('GET /api/profile HTTP/1.1'))

  // 04 Network
  assert.ok(content.includes('04 / NETWORK'))
  assert.ok(content.includes('/images/network.jpg'))
  assert.ok(content.includes('网络与协议'))
  assert.ok(content.includes('9 章节 · 4 次实践'))
  assert.ok(content.includes('SYN → SYN, ACK → ACK'))

  // 05 Reverse
  assert.ok(content.includes('05 / REVERSE ENGINEERING'))
  assert.ok(content.includes('reverse-art'))
  assert.ok(content.includes('二进制与逆向分析'))
  assert.ok(content.includes('14 章节 · 7 次实践'))
  assert.ok(content.includes('call   0x401050 &lt;verify&gt;') || content.includes('call   0x401050 <verify>'))

  // 06 Container
  assert.ok(content.includes('06 / CONTAINER SECURITY'))
  assert.ok(content.includes('container-art'))
  assert.ok(content.includes('HOST KERNEL'))
  assert.ok(content.includes('PID 1'))
  assert.ok(content.includes('CAPS'))
  assert.ok(content.includes('NS'))
  assert.ok(content.includes('容器安全与逃逸'))
  assert.ok(content.includes('11 章节 · 5 次实践'))

  // Auth navigation link
  assert.ok(content.includes('to="/auth"'))
})

test('HeroSection.vue mounts TerminalBackdrop and displays headline and CTA button', () => {
  const content = readFileSync(resolve(rootDir, 'components/landing/HeroSection.vue'), 'utf-8')
  
  assert.ok(content.includes('TerminalBackdrop'))
  assert.ok(content.includes('hero-vignette'))
  assert.ok(content.includes('hero-grain'))
  assert.ok(content.includes('理解原理。'))
  assert.ok(content.includes('亲手验证。'))
  assert.ok(content.includes('to="/auth"'))
  assert.ok(content.includes('开始学习'))
})

test('layouts/landing.vue defines header, slot, and footer with brand logo and clean footer', () => {
  const content = readFileSync(resolve(rootDir, 'layouts/landing.vue'), 'utf-8')
  
  // Header
  assert.ok(content.includes('<header class="header">'))
  assert.ok(content.includes('/logo.png'))
  assert.ok(content.includes('AttackBox'))
  assert.ok(content.includes('to="/auth"'))
  assert.ok(content.includes('进入平台'))

  // Body slot
  assert.ok(content.includes('<slot />') || content.includes('<slot>'))

  // Footer
  assert.ok(content.includes('<footer class="footer">'))
  assert.ok(content.includes('© 2026 AttackBox'))
  assert.ok(!content.includes('GitHub ↗'))
  assert.ok(!content.includes('学习模块'))
})

test('pages/index.vue sets landing layout and mounts hero, section head, and modules', () => {
  const content = readFileSync(resolve(rootDir, 'pages/index.vue'), 'utf-8')
  
  assert.ok(content.includes("layout: 'landing'"))
  assert.ok(content.includes('HeroSection'))
  assert.ok(content.includes('FeaturedModule'))
  assert.ok(content.includes('ModuleRail'))
  assert.ok(content.includes('id="modules"'))
  assert.ok(content.includes('modules wrap'))
  assert.ok(content.includes('下一步，动手。'))
})

test('.output/public/index.html contains full prerendered HTML document with landing sections', () => {
  const outputPath = resolve(rootDir, '.output/public/index.html')
  assert.ok(existsSync(outputPath), '.output/public/index.html should exist after nuxt generate')
  const html = readFileSync(outputPath, 'utf-8')

  assert.ok(html.includes('<title>AttackBox — 理解原理，亲手验证。</title>'))
  assert.ok(html.includes('理解原理。'))
  assert.ok(html.includes('亲手验证。'))
  assert.ok(html.includes('03 / CLOUD NATIVE'))
  assert.ok(html.includes('CLUSTER'))
  assert.ok(html.includes('01 / SYSTEM'))
  assert.ok(html.includes('02 / WEB SECURITY'))
  assert.ok(html.includes('04 / NETWORK'))
  assert.ok(html.includes('05 / REVERSE ENGINEERING'))
  assert.ok(html.includes('06 / CONTAINER SECURITY'))
  assert.ok(html.includes('© 2026 AttackBox'))
})

