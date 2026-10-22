import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, existsSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = fileURLToPath(new URL('.', import.meta.url))
const rootDir = resolve(__dirname, '..')

test('auth components, layout, and page files exist', () => {
  const requiredFiles = [
    'components/auth/AuthVisualAside.vue',
    'components/auth/LoginForm.vue',
    'components/auth/RegisterForm.vue',
    'components/auth/RecoverForm.vue',
    'layouts/auth.vue',
    'pages/auth.vue'
  ]

  for (const file of requiredFiles) {
    const fullPath = resolve(rootDir, file)
    assert.ok(existsSync(fullPath), `File should exist: ${file}`)
    const content = readFileSync(fullPath, 'utf-8')
    assert.ok(content.length > 0, `File should not be empty: ${file}`)
    assert.ok(content.includes('<template>'), `File should be a Vue SFC: ${file}`)
  }
})

test('AuthVisualAside.vue matches visual design with mission taglines and background image', () => {
  const content = readFileSync(resolve(rootDir, 'components/auth/AuthVisualAside.vue'), 'utf-8')

  // Aside container & a11y
  assert.ok(content.includes('<aside') && content.includes('class="visual"'))
  assert.ok(content.includes('aria-label="学习与探索"'))

  // Visual text & tagline
  assert.ok(content.includes('class="visual-text"'))
  assert.ok(content.includes('STAY CURIOUS.'))
  assert.ok(content.includes('让好奇，<span>有迹可循。</span>'))
  assert.ok(content.includes('从一个问题开始，<br>在实践中找到自己的答案。') || content.includes('从一个问题开始，<br />在实践中找到自己的答案。'))

  // Visual footer
  assert.ok(content.includes('class="visual-foot"'))
  assert.ok(content.includes('LEARN. EXPLORE. VERIFY.'))
  assert.ok(content.includes('ATTACKBOX'))

  // Background image
  assert.ok(content.includes('visual-bg') || content.includes('visual-backdrop'))
  assert.ok(content.includes('/images/auth-explore.webp') || content.includes('/images/auth-explore.png'))
  assert.ok(content.includes('blur('))

  // Styling isolation / rules
  assert.ok(content.includes('@media'))
})

test('LoginForm.vue defines credentials validation, ui components, and preview success state', () => {
  const content = readFileSync(resolve(rootDir, 'components/auth/LoginForm.vue'), 'utf-8')

  // Props and emits
  assert.ok(content.includes('modelValueEmail'))
  assert.ok(content.includes('update:modelValueEmail'))
  assert.ok(content.includes('switch-mode'))

  // Consumes UI components
  assert.ok(content.includes('Input'))
  assert.ok(content.includes('Checkbox'))
  assert.ok(content.includes('Button'))
  assert.ok(content.includes('Separator'))

  // Copywriting
  assert.ok(content.includes('YOUR NEXT DISCOVERY'))
  assert.ok(content.includes('欢迎回来。'))
  assert.ok(content.includes('登录，继续你的学习与探索。'))
  assert.ok(content.includes('忘记密码？'))
  assert.ok(content.includes('保持登录状态'))
  assert.ok(content.includes('或使用以下方式'))
  assert.ok(content.includes('GitHub'))
  assert.ok(content.includes('单点登录'))
  assert.ok(content.includes('还没有账号？'))
  assert.ok(content.includes('创建账号 ↗') || content.includes('创建账号'))

  // Success preview view
  assert.ok(content.includes('LOGIN SUCCESS') || content.includes('LOGIN PREVIEW'))
  assert.ok(content.includes('准备好，继续探索。'))
  assert.ok(content.includes('进入学习平台') || content.includes('进入学习原型'))
  assert.ok(content.includes('← 返回登录') || content.includes('返回登录'))

  // Validation logic
  assert.ok(content.includes('请输入有效的邮箱地址。'))
  assert.ok(content.includes('请输入密码。'))
  assert.ok(content.includes('密码至少需要 6 位。'))
})

test('RegisterForm.vue renders registration fields, hints, and prototype preview', () => {
  const content = readFileSync(resolve(rootDir, 'components/auth/RegisterForm.vue'), 'utf-8')

  // Props and emits
  assert.ok(content.includes('modelValueEmail'))
  assert.ok(content.includes('update:modelValueEmail'))
  assert.ok(content.includes('switch-mode'))

  // Copywriting
  assert.ok(content.includes('ATTACKBOX ACCOUNT'))
  assert.ok(content.includes('从这里，开始。'))
  assert.ok(content.includes('创建账号，开始你的下一次探索。'))
  assert.ok(content.includes('继续创建账号'))
  assert.ok(content.includes('下一步验证邮箱，完成账号创建。'))
  assert.ok(content.includes('邮箱验证邮件已发送') || content.includes('邮箱验证步骤预览'))
  assert.ok(content.includes('← 返回登录') || content.includes('返回登录'))

  // Password placeholder
  assert.ok(content.includes('设置密码，至少 6 位'))
})

test('RecoverForm.vue renders recovery form and prototype notification', () => {
  const content = readFileSync(resolve(rootDir, 'components/auth/RecoverForm.vue'), 'utf-8')

  // Props and emits
  assert.ok(content.includes('modelValueEmail'))
  assert.ok(content.includes('update:modelValueEmail'))
  assert.ok(content.includes('switch-mode'))

  // Copywriting
  assert.ok(content.includes('ATTACKBOX ACCOUNT'))
  assert.ok(content.includes('找回密码。'))
  assert.ok(content.includes('输入注册邮箱，我们将帮助你重设密码。'))
  assert.ok(content.includes('发送重置链接'))
  assert.ok(content.includes('重置密码链接已发送') || content.includes('重置链接发送状态预览'))
  assert.ok(content.includes('← 返回登录') || content.includes('返回登录'))
})

test('layouts/auth.vue defines isolated layout, header with brand, home-link, slot, and footer', () => {
  const content = readFileSync(resolve(rootDir, 'layouts/auth.vue'), 'utf-8')

  // Scoped / isolated layout class
  assert.ok(content.includes('auth-layout'))

  // Header elements
  assert.ok(content.includes('/logo.png'))
  assert.ok(content.includes('AttackBox'))
  assert.ok(content.includes('home-link'))
  assert.ok(content.includes('返回首页'))

  // Main slot
  assert.ok(content.includes('<slot />') || content.includes('<slot>'))

  // Footer
  assert.ok(content.includes('© 2026 AttackBox'))
  assert.ok(!content.includes('交互原型 · 不会提交账号信息'))
})

test('pages/auth.vue binds auth layout, handles mode query and title, and syncs email state', () => {
  const content = readFileSync(resolve(rootDir, 'pages/auth.vue'), 'utf-8')

  // Layout metadata
  assert.ok(content.includes("layout: 'auth'"))

  // Head configuration
  assert.ok(content.includes('useHead'))
  assert.ok(content.includes('登录 — AttackBox'))
  assert.ok(content.includes('注册 — AttackBox'))
  assert.ok(content.includes('找回密码 — AttackBox'))

  // Aside & Forms mounting
  assert.ok(content.includes('AuthVisualAside'))
  assert.ok(content.includes('LoginForm'))
  assert.ok(content.includes('RegisterForm'))
  assert.ok(content.includes('RecoverForm'))

  // Mode switching & state synchronization
  assert.ok(content.includes('navigateTo'))
  assert.ok(content.includes('route.query.mode') || content.includes('query'))
})
