// web/tests/learning-ui.test.js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const webDir = path.resolve(import.meta.dirname || path.dirname(fileURLToPath(import.meta.url)), '..');

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

test('pages/learn/modules/[slug]/index.vue renders real module content and configurable learning outcomes', () => {
  const content = fs.readFileSync(path.join(webDir, 'pages/learn/modules/[slug]/index.vue'), 'utf-8');
  assert.ok(content.includes("layout: 'learning'"));
  assert.ok(content.includes('/api/modules/'));
  assert.ok(content.includes('hero-section'));
  assert.ok(content.includes('module-content'));
  assert.ok(content.includes('moduleData.learningOutcomes'));
  assert.ok(!content.includes('progress-pct'));
  assert.ok(!content.includes('关于章节测验'));
  assert.ok(!content.includes('循序渐进'));
  assert.ok(content.includes('lucide-vue-next'));
  assert.ok(content.includes('chapter-number'));
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
  // 阅读端必须适配后端嵌套结构,否则标题/正文会渲染成 undefined
  assert.ok(content.includes('data.chapter.title'), 'chapter page must read data.chapter');
  assert.ok(content.includes('data.questions'), 'chapter page must read top-level questions');
  assert.ok(content.includes('/attempt'), 'chapter page must post answers to the attempt endpoint');
  assert.ok(content.includes('renderPrompt'), 'question prompts must render through sanitized markdown');
});

test('reader pages map the nested API payload (module/chapter)', () => {
  const modulePage = fs.readFileSync(path.join(webDir, 'pages/learn/modules/[slug]/index.vue'), 'utf-8');
  assert.ok(modulePage.includes('data.module'), 'roadmap page must read data.module');
  assert.ok(modulePage.includes('chapters: data.chapters'), 'roadmap page must lift data.chapters');
  const chapterPage = fs.readFileSync(path.join(webDir, 'pages/learn/modules/[slug]/[chapterSlug].vue'), 'utf-8');
  assert.ok(chapterPage.includes('data.module'), 'chapter page must read data.module for the module title');
});

test('quiz attempt endpoint verifies answers server-side without leaking hashes', () => {
  const route = fs.readFileSync(path.join(webDir, 'server/api/modules/[slug]/chapters/[chapterSlug]/attempt.post.ts'), 'utf-8');
  assert.ok(route.includes('verifyQuizAnswer'));
  assert.ok(route.includes('questionId'));
  const responseBlock = route.slice(route.indexOf('return {'));
  assert.ok(responseBlock.includes('correct'));
  assert.ok(responseBlock.includes('errorHint'));
  assert.ok(!responseBlock.includes('answerHmac'), 'response must not leak answer_hmac');
});

test('nuxt.config injects a pre-paint theme bootstrap to avoid the theme flash', () => {
  const content = fs.readFileSync(path.join(webDir, 'nuxt.config.ts'), 'utf-8');
  assert.ok(content.includes('attackbox-appearance'));
  assert.ok(content.includes('data-theme'));
});
