// web/tests/admin-ui.test.js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const webDir = path.resolve(import.meta.dirname || path.dirname(fileURLToPath(import.meta.url)), '..');

test('admin layout and page templates exist', () => {
  const files = [
    'layouts/admin.vue',
    'pages/admin/modules/index.vue',
    'pages/admin/modules/[id].vue',
  ];

  for (const f of files) {
    const fullPath = path.join(webDir, f);
    assert.ok(fs.existsSync(fullPath), `File should exist: ${f}`);
  }
});

test('layouts/admin.vue defines admin shell, navigation groups, and permission check', () => {
  const content = fs.readFileSync(path.join(webDir, 'layouts/admin.vue'), 'utf-8');
  assert.ok(content.includes('admin-shell'));
  assert.ok(content.includes('src="/logo.png"'));
  assert.ok(content.includes('AttackBox Admin'));
  assert.ok(content.includes('内容管理'));
  assert.ok(content.includes('账户管理'));
  assert.ok(content.includes('环境管理'));
  assert.ok(content.includes('系统管理'));
  assert.ok(content.includes("role === 'admin'"));
  assert.ok(content.includes('admin-denied'));
});

test('pages/admin/modules/index.vue defines module list, status filters, and unsaved creation route', () => {
  const content = fs.readFileSync(path.join(webDir, 'pages/admin/modules/index.vue'), 'utf-8');
  assert.ok(content.includes("layout: 'admin'"));
  assert.ok(content.includes('/api/admin/modules'));
  assert.ok(content.includes('module-table'));
  assert.ok(content.includes('/admin/modules/new'));
  assert.ok(!content.includes('handleCreateModule'));
  assert.ok(content.includes('filters-tabs'));
});

test('pages/admin/modules/[id].vue defines editor tabs, direct saving, publish, and reorder', () => {
  const content = fs.readFileSync(path.join(webDir, 'pages/admin/modules/[id].vue'), 'utf-8');
  assert.ok(content.includes("layout: 'admin'"));
  assert.ok(content.includes('/api/admin/modules/'));
  assert.ok(content.includes('lockVersion'));
  assert.ok(!content.includes('baseRevisionId'));
  assert.ok(content.includes('chapters: localChapters.value.map'));
  assert.ok(!content.includes('handleDiscardDraft'));
  assert.ok(content.includes('保存为草稿'));
  assert.ok(content.includes('保存并发布'));
  assert.ok(content.includes('module-outcomes'));
  assert.ok(content.includes('handleDeleteModule'));
  assert.ok(content.includes('editor-double-layout'));
  assert.ok(content.includes('MarkdownView'));
  assert.ok(!content.includes('handlePublish'));
  assert.ok(!content.includes('/publish`'));
  assert.ok(content.includes('handleSave'));
  assert.ok(content.includes('handleMoveChapter'));
  assert.ok(content.includes('handleAddQuestion'));
  assert.ok(content.includes('handleRemoveChapter'));
  assert.ok(!content.includes('预览大纲'));
  assert.ok(!content.includes('isPublishErrorModalOpen'));
  assert.ok(content.indexOf("activeTab === 'basic'") < content.indexOf("activeTab === 'chapters'"));
  assert.ok(content.includes('chapter-delete-btn'));
});
