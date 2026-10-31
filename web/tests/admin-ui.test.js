// web/tests/admin-ui.test.js
import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const webDir = path.resolve(
  import.meta.dirname || path.dirname(fileURLToPath(import.meta.url)),
  '..'
)

test('admin layout and page templates exist and legacy editor is removed', () => {
  const files = [
    'layouts/admin.vue',
    'pages/admin/modules/index.vue',
    'pages/admin/modules/[id].vue'
  ]

  for (const f of files) {
    const fullPath = path.join(webDir, f)
    assert.ok(fs.existsSync(fullPath), `File should exist: ${f}`)
  }
  assert.ok(!fs.existsSync(path.join(webDir, 'components/admin/LegacyModuleEditor.vue')))
  assert.ok(!fs.existsSync(path.join(webDir, 'components/admin/CategorySelect.vue')))
  assert.ok(!fs.existsSync(path.join(webDir, 'server/api/admin/modules/index.post.ts')))
  assert.ok(!fs.existsSync(path.join(webDir, 'server/api/admin/modules/[id]/index.put.ts')))
})

test('layouts/admin.vue defines admin shell, navigation groups, and permission check', () => {
  const content = fs.readFileSync(path.join(webDir, 'layouts/admin.vue'), 'utf-8')
  assert.ok(content.includes('admin-shell'))
  assert.ok(content.includes('src="~/assets/images/logo.webp"'))
  assert.ok(content.includes('AttackBox Admin'))
  assert.ok(content.includes('内容管理'))
  assert.ok(content.includes('账户管理'))
  assert.ok(content.includes('环境管理'))
  assert.ok(content.includes('系统管理'))
  assert.ok(content.includes("role === 'admin'"))
  assert.ok(content.includes('admin-denied'))
})

test('pages/admin/modules/index.vue defines module list, status filters, and manual Git entry', () => {
  const content = fs.readFileSync(path.join(webDir, 'pages/admin/modules/index.vue'), 'utf-8')
  assert.ok(content.includes("layout: 'admin'"))
  assert.ok(content.includes('/api/admin/modules'))
  assert.ok(content.includes('module-table'))
  assert.ok(content.includes('/admin/content'))
  assert.ok(content.includes('/api/admin/content/template'))
  assert.ok(!content.includes('/admin/modules/new'))
  assert.ok(!content.includes('handleCreateModule'))
  assert.ok(!content.includes('编辑旧模块'))
  assert.ok(content.includes('filters-tabs'))
})

test('module route renders GitModuleDetail directly without legacy editor fallback', () => {
  const content = fs.readFileSync(path.join(webDir, 'pages/admin/modules/[id].vue'), 'utf-8')
  assert.ok(content.includes('GitModuleDetail'))
  assert.ok(!content.includes('LegacyModuleEditor'))
  assert.ok(!content.includes('defineAsyncComponent'))
  assert.ok(content.includes("navigateTo('/admin/content'"))
})
