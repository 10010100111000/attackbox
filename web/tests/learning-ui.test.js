// web/tests/learning-ui.test.js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const webDir = path.resolve(import.meta.dirname, '..');

test('learning components, layouts, and page templates exist', () => {
  const files = [
    'layouts/learning.vue',
    'components/learning/LearningHeader.vue',
    'components/ui/MarkdownView.vue',
    'composables/useTheme.ts',
    'pages/learn/index.vue',
    'pages/learn/modules/[slug]/index.vue',
    'pages/learn/modules/[slug]/[chapterSlug].vue',
  ];

  for (const f of files) {
    const fullPath = path.join(webDir, f);
    assert.ok(fs.existsSync(fullPath), `File should exist: ${f}`);
  }
});

test('LearningHeader.vue uses /logo.png, AttackBox brand, and navigation links', () => {
  const content = fs.readFileSync(path.join(webDir, 'components/learning/LearningHeader.vue'), 'utf-8');
  assert.ok(content.includes('src="/logo.png"'), 'Must use /logo.png as requested');
  assert.ok(content.includes('AttackBox'), 'Must display AttackBox brand name');
  assert.ok(content.includes('to="/learn"'), 'Must link to /learn');
  assert.ok(content.includes('to="/challenges"'), 'Must link to /challenges');
  assert.ok(content.includes('to="/discuss"'), 'Must link to /discuss');
  assert.ok(content.includes('appearance-switch'), 'Must provide theme switcher');
});

test('pages/learn/index.vue defines learning layout, /api/modules fetch, and categories', () => {
  const content = fs.readFileSync(path.join(webDir, 'pages/learn/index.vue'), 'utf-8');
  assert.ok(content.includes("layout: 'learning'"));
  assert.ok(content.includes('/api/modules'));
  assert.ok(content.includes('feature-banner'));
  assert.ok(content.includes('carousel-pagination'));
  assert.ok(content.includes('page-tabs'));
  assert.ok(content.includes('cards'));
  assert.ok(content.includes('card-art'));
});

test('pages/learn/modules/[slug]/index.vue renders roadmap layout and stats', () => {
  const content = fs.readFileSync(path.join(webDir, 'pages/learn/modules/[slug]/index.vue'), 'utf-8');
  assert.ok(content.includes("layout: 'learning'"));
  assert.ok(content.includes('/api/modules/'));
  assert.ok(content.includes('hero-section'));
  assert.ok(content.includes('road-layout'));
  assert.ok(content.includes('step-circle'));
  assert.ok(content.includes('start-learning-btn'));
});

test('pages/learn/modules/[slug]/[chapterSlug].vue renders learn-shell, MarkdownView, and quiz', () => {
  const content = fs.readFileSync(path.join(webDir, 'pages/learn/modules/[slug]/[chapterSlug].vue'), 'utf-8');
  assert.ok(content.includes("layout: 'learning'"));
  assert.ok(content.includes('learn-shell'));
  assert.ok(content.includes('MarkdownView'));
  assert.ok(content.includes('quiz-card'));
  assert.ok(content.includes('answer-check-btn'));
  assert.ok(content.includes('auth-gate-card'));
});
