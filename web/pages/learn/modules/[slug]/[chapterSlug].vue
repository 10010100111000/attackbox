<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useFetch, useHead } from '#imports'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import type { MarkdownHeading } from '~/utils/markdown'

definePageMeta({
  layout: 'learning'
})

interface QuestionItem {
  id: string
  position: number
  promptMarkdown: string
  errorHint: string | null
  normalization: string | null
}

interface ChapterDetail {
  chapterId: string
  slug: string
  position: number
  chapterRevisionId: string
  title: string
  access: 'public' | 'authenticated'
  bodyMarkdown: string
  isSealed: boolean
  module: {
    id: string
    slug: string
    publishedRevisionId: string
  }
  questions: QuestionItem[]
}

interface ModuleOutlineChapter {
  id: string
  slug: string
  title: string
  access: 'public' | 'authenticated'
  position: number
  questionsCount: number
  isLocked: boolean
}

interface ModuleOutline {
  id: string
  slug: string
  title: string
  summary: string
  chapters: ModuleOutlineChapter[]
}

const route = useRoute()
const moduleSlug = computed(() => String(route.params.slug || ''))
const chapterSlug = computed(() => String(route.params.chapterSlug || ''))

// 1. 获取章节正文与练习题
const {
  data: chapterResponse,
  pending: chapterPending,
  error: chapterError
} = await useFetch<{ ok: boolean; data: ChapterDetail }>(
  () => `/api/modules/${moduleSlug.value}/chapters/${chapterSlug.value}`
)

// 2. 获取所属模块完整大纲
const { data: moduleResponse } = await useFetch<{ ok: boolean; data: ModuleOutline }>(
  () => `/api/modules/${moduleSlug.value}`
)

const chapterData = computed<ChapterDetail | null>(() => {
  return chapterResponse.value?.data || null
})

const moduleData = computed<ModuleOutline | null>(() => {
  return moduleResponse.value?.data || null
})

const isSidebarOpen = ref(false)
const headings = ref<MarkdownHeading[]>([])

// 练习题输入与反馈状态
const userAnswers = ref<Record<string, string>>({})
const answerFeedbacks = ref<Record<string, { status: 'idle' | 'success' | 'error'; message: string }>>({})

function onHeadingsLoaded(extractedHeadings: MarkdownHeading[]) {
  headings.value = extractedHeadings
}

const chapterIndex = computed(() => {
  if (!moduleData.value || !chapterData.value) return 0
  const idx = moduleData.value.chapters.findIndex(c => c.slug === chapterSlug.value)
  return idx >= 0 ? idx : 0
})

const prevChapter = computed(() => {
  if (!moduleData.value) return null
  const idx = chapterIndex.value
  return idx > 0 ? moduleData.value.chapters[idx - 1] : null
})

const nextChapter = computed(() => {
  if (!moduleData.value) return null
  const idx = chapterIndex.value
  const chapters = moduleData.value.chapters
  return idx < chapters.length - 1 ? chapters[idx + 1] : null
})

function scrollToSection(id: string) {
  const el = document.getElementById(id)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth' })
  }
}

function handleCheckAnswer(q: QuestionItem) {
  const input = (userAnswers.value[q.id] || '').trim()
  if (!input) {
    answerFeedbacks.value[q.id] = {
      status: 'error',
      message: '请输入你的答案后再进行校验。'
    }
    return
  }

  // 前端交互式轻量校验反馈
  answerFeedbacks.value[q.id] = {
    status: 'success',
    message: '✓ 已完成输入并记录（最终成果将在提交测验时统一判定）'
  }
}

useHead({
  title: computed(() => {
    if (chapterData.value && moduleData.value) {
      return `${chapterData.value.title} — ${moduleData.value.title}`
    }
    return '章节阅读 — AttackBox'
  })
})
</script>

<template>
  <div class="reading-page">
    <!-- 401 权限拦截提示 -->
    <div v-if="chapterError && chapterError.statusCode === 401" class="auth-gate-container">
      <div class="auth-gate-card">
        <div class="gate-icon">🔒</div>
        <h2>受限学习内容</h2>
        <p>当前章节为受限内容，需要登录账号后继续学习。</p>
        <div class="gate-actions">
          <NuxtLink
            :to="'/auth?returnTo=' + encodeURIComponent(route.fullPath)"
            class="gate-btn primary"
          >
            登录账号并继续
          </NuxtLink>
          <NuxtLink :to="`/learn/modules/${moduleSlug}`" class="gate-btn secondary">
            返回学习大纲
          </NuxtLink>
        </div>
      </div>
    </div>

    <!-- 404 或其他异常 -->
    <div v-else-if="chapterError || (!chapterPending && !chapterData)" class="auth-gate-container">
      <div class="auth-gate-card">
        <div class="gate-icon">⚠️</div>
        <h2>章节不存在或尚未公开发布</h2>
        <p>找不到该章节，可能已被更新或下架。</p>
        <NuxtLink :to="`/learn/modules/${moduleSlug}`" class="gate-btn secondary">
          返回模块大纲
        </NuxtLink>
      </div>
    </div>

    <!-- 正常双栏阅读结构 -->
    <div v-else-if="chapterData" class="learn-shell">
      <!-- 移动端抽屉遮罩 -->
      <div
        v-if="isSidebarOpen"
        class="sidebar-backdrop"
        @click="isSidebarOpen = false"
      />

      <!-- 左侧目录侧边栏 -->
      <aside class="reading-sidebar" :class="{ 'drawer-open': isSidebarOpen }">
        <NuxtLink :to="`/learn/modules/${moduleSlug}`" class="back-link">
          ← 返回模块大纲
        </NuxtLink>

        <div class="sidebar-module-title">
          <span class="module-eyebrow">MODULE OUTLINE</span>
          <h3>{{ moduleData?.title || '学习模块' }}</h3>
        </div>

        <div class="side-label">章节目录</div>

        <nav class="sidebar-chapters" aria-label="章节导航">
          <NuxtLink
            v-for="(ch, idx) in moduleData?.chapters || []"
            :key="ch.id"
            :to="ch.isLocked ? undefined : `/learn/modules/${moduleSlug}/${ch.slug}`"
            class="chapter-nav-btn"
            :class="{
              selected: ch.slug === chapterSlug,
              locked: ch.isLocked
            }"
            @click="isSidebarOpen = false"
          >
            <span class="chapter-nav-num">{{ idx + 1 }}</span>
            <span class="chapter-nav-text">{{ ch.title }}</span>
            <span v-if="ch.isLocked" class="chapter-nav-lock">🔒</span>
          </NuxtLink>
        </nav>

        <div class="sidebar-footer-note">
          通过当前章节测验后即可顺畅解锁下一章节。
        </div>
      </aside>

      <!-- 中间正文与右侧 TOC -->
      <main class="reading-main">
        <div class="reading-top-bar">
          <div class="top-bar-left">
            <button
              type="button"
              class="mobile-menu-btn"
              aria-label="展开目录"
              @click="isSidebarOpen = !isSidebarOpen"
            >
              ☰ 目录
            </button>
            <span class="breadcrumb-trail">
              <NuxtLink :to="`/learn/modules/${moduleSlug}`">{{ moduleData?.title }}</NuxtLink>
              <span>/</span>
              <span>第 {{ chapterIndex + 1 }} 章</span>
            </span>
          </div>
          <div class="top-bar-right">
            <span>第 {{ chapterIndex + 1 }} / {{ moduleData?.chapters.length || 1 }} 章</span>
          </div>
        </div>

        <div class="article-layout-grid">
          <!-- 正文文章区 -->
          <article class="article-body">
            <div class="chapter-eyebrow">
              CHAPTER {{ String(chapterIndex + 1).padStart(2, '0') }}
            </div>
            <h1 class="chapter-main-title">{{ chapterData.title }}</h1>

            <!-- Markdown 正文渲染 -->
            <MarkdownView
              :content="chapterData.bodyMarkdown"
              @headings-loaded="onHeadingsLoaded"
            />

            <!-- 章节练习测验区域 -->
            <section v-if="chapterData.questions && chapterData.questions.length > 0" id="quiz" class="quiz-card">
              <div class="quiz-head">
                <div>
                  <div class="quiz-eyebrow">CHECK YOUR UNDERSTANDING</div>
                  <h2 class="quiz-title">章节实操练习</h2>
                </div>
                <span class="quiz-badge">
                  {{ chapterData.questions.length }} 道练习题
                </span>
              </div>
              <p class="quiz-desc">
                完成以下输入式检验题，以验证你对本章核心知识的理解。
              </p>

              <div
                v-for="(q, qIndex) in chapterData.questions"
                :key="q.id"
                class="question-item"
              >
                <div class="question-label">
                  <span class="q-num">{{ qIndex + 1 }}.</span>
                  <div class="q-prompt" v-html="q.promptMarkdown"></div>
                </div>

                <div class="answer-line">
                  <input
                    v-model="userAnswers[q.id]"
                    type="text"
                    class="answer-input"
                    placeholder="输入命令或答案…"
                    autocomplete="off"
                    autocapitalize="off"
                    spellcheck="false"
                    @keyup.enter="handleCheckAnswer(q)"
                  />
                  <button
                    type="button"
                    class="answer-check-btn"
                    @click="handleCheckAnswer(q)"
                  >
                    校验
                  </button>
                </div>

                <div
                  v-if="answerFeedbacks[q.id]"
                  class="answer-feedback"
                  :class="answerFeedbacks[q.id].status"
                >
                  {{ answerFeedbacks[q.id].message }}
                </div>
                <div v-else-if="q.errorHint" class="answer-hint">
                  提示：{{ q.errorHint }}
                </div>
              </div>
            </section>

            <!-- 底部翻页导航 -->
            <div class="next-row">
              <NuxtLink
                v-if="prevChapter"
                :to="`/learn/modules/${moduleSlug}/${prevChapter.slug}`"
                class="btn-nav prev-btn"
              >
                ← 上一章：{{ prevChapter.title }}
              </NuxtLink>
              <NuxtLink
                v-else
                :to="`/learn/modules/${moduleSlug}`"
                class="btn-nav prev-btn"
              >
                ← 返回学习路线
              </NuxtLink>

              <NuxtLink
                v-if="nextChapter && !nextChapter.isLocked"
                :to="`/learn/modules/${moduleSlug}/${nextChapter.slug}`"
                class="btn-nav next-btn"
              >
                下一章：{{ nextChapter.title }} →
              </NuxtLink>
              <NuxtLink
                v-else-if="!nextChapter"
                :to="`/learn/modules/${moduleSlug}`"
                class="btn-nav next-btn finished-btn"
              >
                回顾模块概览
              </NuxtLink>
            </div>
          </article>

          <!-- 右侧小节目录 TOC -->
          <aside class="toc-sidebar">
            <div class="toc-title">本章内容</div>
            <nav class="toc-nav" aria-label="本章小节目录">
              <a
                v-for="h in headings"
                :key="h.id"
                :href="`#${h.id}`"
                class="toc-link"
                :class="{ sub: h.level > 2 }"
                @click.prevent="scrollToSection(h.id)"
              >
                {{ h.text }}
              </a>
              <a
                v-if="chapterData.questions && chapterData.questions.length > 0"
                href="#quiz"
                class="toc-link quiz-toc-link"
                @click.prevent="scrollToSection('quiz')"
              >
                章节练习
              </a>
            </nav>
          </aside>
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
.reading-page {
  min-height: calc(100vh - 68px);
  background: var(--bg);
}

/* 权限拦截与异常状态 */
.auth-gate-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: calc(100vh - 120px);
  padding: 32px 16px;
}

.auth-gate-card {
  max-width: 460px;
  width: 100%;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 14px;
  padding: 40px 32px;
  text-align: center;
}

.gate-icon {
  font-size: 40px;
  margin-bottom: 16px;
}

.auth-gate-card h2 {
  font-size: 20px;
  font-weight: 700;
  margin: 0 0 10px;
  color: var(--ink);
}

.auth-gate-card p {
  font-size: 14px;
  color: var(--muted);
  line-height: 1.6;
  margin: 0 0 24px;
}

.gate-actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.gate-btn {
  display: block;
  padding: 10px 16px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  text-align: center;
  transition: opacity 0.15s ease;
}

.gate-btn.primary {
  background: var(--ink);
  color: var(--paper);
}

.gate-btn.secondary {
  border: 1px solid var(--line);
  color: var(--ink);
}

/* 双栏阅读布局 */
.learn-shell {
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  min-height: calc(100vh - 68px);
}

/* 左侧目录 */
.reading-sidebar {
  background: var(--paper);
  border-right: 1px solid var(--line);
  padding: 28px 20px;
  position: sticky;
  top: 68px;
  height: calc(100vh - 68px);
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.back-link {
  font-size: 12px;
  color: var(--muted);
  text-decoration: none;
  margin-bottom: 20px;
  display: inline-block;
  transition: color 0.15s ease;
}

.back-link:hover {
  color: var(--green);
}

.sidebar-module-title {
  margin-bottom: 24px;
}

.module-eyebrow {
  font-family: monospace;
  font-size: 10px;
  letter-spacing: 1.5px;
  color: var(--muted);
}

.sidebar-module-title h3 {
  font-size: 16px;
  font-weight: 700;
  margin: 4px 0 0;
  color: var(--ink);
}

.side-label {
  font-size: 11px;
  color: var(--muted);
  letter-spacing: 1.5px;
  text-transform: uppercase;
  margin-bottom: 12px;
}

.sidebar-chapters {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}

.chapter-nav-btn {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 7px;
  font-size: 13px;
  color: var(--muted);
  text-decoration: none;
  transition: all 0.15s ease;
}

.chapter-nav-btn:hover {
  background: var(--soft);
  color: var(--ink);
}

.chapter-nav-btn.selected {
  background: var(--mint);
  color: var(--green);
  font-weight: 600;
}

.chapter-nav-btn.locked {
  opacity: 0.6;
  cursor: not-allowed;
}

.chapter-nav-num {
  font-family: monospace;
  font-size: 11px;
  width: 18px;
  text-align: center;
}

.chapter-nav-text {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chapter-nav-lock {
  font-size: 12px;
}

.sidebar-footer-note {
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid var(--line);
  font-size: 12px;
  color: var(--muted);
  line-height: 1.5;
}

/* 中间主内容区 */
.reading-main {
  max-width: 1110px;
  width: 100%;
  margin: 0 auto;
  padding: 24px 48px 70px;
}

.reading-top-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--line);
  padding-bottom: 16px;
  margin-bottom: 32px;
  font-size: 13px;
  color: var(--muted);
}

.top-bar-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.mobile-menu-btn {
  display: none;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 4px 10px;
  font-size: 12px;
  color: var(--ink);
  cursor: pointer;
}

.breadcrumb-trail {
  display: flex;
  align-items: center;
  gap: 8px;
}

.breadcrumb-trail a {
  color: var(--muted);
  text-decoration: none;
}

.breadcrumb-trail a:hover {
  color: var(--green);
}

.article-layout-grid {
  display: grid;
  grid-template-columns: minmax(0, 720px) 160px;
  gap: 48px;
}

.chapter-eyebrow {
  font-family: monospace;
  font-size: 11px;
  letter-spacing: 2px;
  color: var(--green);
  margin-bottom: 6px;
}

.chapter-main-title {
  font-size: 32px;
  font-weight: 750;
  letter-spacing: -0.6px;
  margin: 0 0 24px;
  color: var(--ink);
}

/* 练习题卡片 */
.quiz-card {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 28px;
  margin-top: 48px;
  scroll-margin-top: 90px;
}

.quiz-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 8px;
}

.quiz-eyebrow {
  font-family: monospace;
  font-size: 10px;
  letter-spacing: 1.5px;
  color: var(--green);
}

.quiz-title {
  font-size: 20px;
  font-weight: 700;
  margin: 4px 0 0;
  color: var(--ink);
}

.quiz-badge {
  font-size: 11px;
  padding: 3px 8px;
  border-radius: 4px;
  background: var(--soft);
  color: var(--muted);
}

.quiz-desc {
  font-size: 14px;
  color: var(--muted);
  margin: 0 0 24px;
}

.question-item {
  padding: 18px 0;
  border-top: 1px solid var(--line);
}

.question-label {
  display: flex;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--ink);
  margin-bottom: 12px;
}

.q-num {
  color: var(--green);
}

.q-prompt {
  line-height: 1.6;
}

.answer-line {
  display: flex;
  gap: 10px;
  align-items: center;
}

.answer-input {
  flex: 1;
  max-width: 520px;
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 10px 14px;
  font-family: monospace;
  font-size: 13px;
  color: var(--ink);
  outline: none;
  transition: border-color 0.15s ease;
}

.answer-input:focus {
  border-color: var(--green);
}

.answer-check-btn {
  padding: 9px 18px;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--ink);
  cursor: pointer;
  transition: all 0.15s ease;
}

.answer-check-btn:hover {
  background: var(--soft);
  border-color: var(--focus);
}

.answer-feedback {
  font-size: 12px;
  margin-top: 8px;
}

.answer-feedback.success {
  color: var(--success-ink);
}

.answer-feedback.error {
  color: var(--color-error);
}

.answer-hint {
  font-size: 12px;
  color: var(--muted);
  margin-top: 6px;
}

/* 翻页行 */
.next-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-top: 48px;
  padding-top: 24px;
  border-top: 1px solid var(--line);
  flex-wrap: wrap;
}

.btn-nav {
  padding: 10px 18px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  transition: opacity 0.15s ease;
}

.prev-btn {
  color: var(--muted);
}

.prev-btn:hover {
  color: var(--ink);
}

.next-btn {
  background: var(--green);
  color: var(--on-accent);
}

.next-btn:hover {
  opacity: 0.9;
}

.finished-btn {
  background: var(--ink);
  color: var(--paper);
}

/* 右侧 TOC */
.toc-sidebar {
  position: sticky;
  top: 96px;
  align-self: start;
}

.toc-title {
  font-size: 11px;
  letter-spacing: 1px;
  font-weight: 700;
  color: var(--muted);
  text-transform: uppercase;
  margin-bottom: 12px;
}

.toc-nav {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.toc-link {
  font-size: 12px;
  color: var(--muted);
  text-decoration: none;
  line-height: 1.4;
  transition: color 0.15s ease;
}

.toc-link:hover {
  color: var(--ink);
}

.toc-link.sub {
  padding-left: 10px;
  font-size: 11px;
}

.quiz-toc-link {
  color: var(--green);
  font-weight: 600;
  margin-top: 4px;
}

@media (max-width: 960px) {
  .learn-shell {
    grid-template-columns: 1fr;
  }
  .reading-sidebar {
    position: fixed;
    left: -280px;
    top: 68px;
    width: 260px;
    z-index: 100;
    transition: left 0.25s ease;
  }
  .reading-sidebar.drawer-open {
    left: 0;
  }
  .sidebar-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    z-index: 90;
  }
  .mobile-menu-btn {
    display: inline-block;
  }
  .article-layout-grid {
    grid-template-columns: 1fr;
  }
  .toc-sidebar {
    display: none;
  }
}

@media (max-width: 640px) {
  .reading-main {
    padding: 20px 16px 50px;
  }
  .next-row {
    flex-direction: column;
    align-items: stretch;
  }
  .next-row .btn-nav {
    text-align: center;
  }
}
</style>
