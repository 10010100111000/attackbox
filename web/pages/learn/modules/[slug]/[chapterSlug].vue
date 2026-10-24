<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useFetch, useHead } from '#imports'
import { Box, ArrowLeft, ArrowRight, LockKeyhole, PanelLeft, Clock3, Terminal, Monitor, ChevronUp, CheckCircle2, CircleAlert, LoaderCircle, BookOpen } from 'lucide-vue-next'
import { DialogRoot, DialogTrigger, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription } from 'reka-ui'
import ChapterOutline from '~/components/learning/ChapterOutline.vue'
import PracticeDock from '~/components/learning/PracticeDock.vue'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import { useAuth } from '~/composables/useAuth'
import type { LearningTarget } from '~/shared/learning-target'
import type { ChapterProgress, LearningChapter } from '~/shared/learning'
import { renderMarkdown } from '~/utils/markdown'

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

interface ChapterInfo {
  id: string
  slug: string
  title: string
  access: 'public' | 'authenticated'
  position: number
  bodyMarkdown: string
  durationMinutes: number | null
}

// 接口返回 { chapter, questions },这里归一化成页面使用的扁平结构
interface ChapterDetail {
  title: string
  bodyMarkdown: string
  durationMinutes: number | null
  questions: QuestionItem[]
}

type ModuleOutlineChapter = LearningChapter

interface ModuleOutline {
  id: string
  slug: string
  title: string
  summary: string
  chapters: ModuleOutlineChapter[]
}

const route = useRoute()
const { currentUser } = useAuth()
const learnerId = computed(() => currentUser.value?.id || '')
const moduleSlug = computed(() => String(route.params.slug || ''))
const chapterSlug = computed(() => String(route.params.chapterSlug || ''))

// 1. 获取章节正文与练习题
const {
  data: chapterResponse,
  pending: chapterPending,
  error: chapterError
} = await useFetch<{
  ok: boolean
  data: { chapter: ChapterInfo; questions: QuestionItem[]; progress: ChapterProgress; target:LearningTarget }
}>(
  () => `/api/modules/${moduleSlug.value}/chapters/${chapterSlug.value}`,
  { key: `learning:chapter:${moduleSlug.value}:${chapterSlug.value}`, watch: [learnerId] }
)

// 2. 获取所属模块完整大纲
const { data: moduleResponse, refresh: refreshModule } = await useFetch<{
  ok: boolean
  data: { module: Omit<ModuleOutline, 'chapters'>; chapters: ModuleOutlineChapter[] }
}>(
  () => `/api/modules/${moduleSlug.value}`,
  { key: `learning:outline:${moduleSlug.value}`, watch: [learnerId] }
)

const chapterData = computed<ChapterDetail | null>(() => {
  const data = chapterResponse.value?.data
  if (!data) return null
  return {
    title: data.chapter.title,
    bodyMarkdown: data.chapter.bodyMarkdown,
    durationMinutes: data.chapter.durationMinutes,
    questions: data.questions || []
  }
})

const moduleData = computed<ModuleOutline | null>(() => {
  const data = moduleResponse.value?.data
  if (!data) return null
  return { ...data.module, chapters: data.chapters }
})

const sidebarCollapsed = ref(false)
const mobileOutlineOpen = ref(false)
const scrollElement = ref<HTMLElement | null>(null)
const practiceMode = ref<'cli' | 'gui'>('cli')
const practiceOpened = ref(false)
const practiceVisible = ref(false), practiceMinimized = ref(false), practiceExpanded = ref(false)
const dockHeight = ref(310), viewportHeight = ref(900)
const maxDockHeight = computed(() => Math.max(180, viewportHeight.value - 220))
function updateViewport() {
  viewportHeight.value = window.innerHeight
  dockHeight.value = Math.min(dockHeight.value, maxDockHeight.value)
  if (window.innerWidth > 900) mobileOutlineOpen.value = false
}
function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  try { localStorage.setItem('attackbox-reader-outline-collapsed', String(sidebarCollapsed.value)) } catch {}
}
function openPractice(mode: 'cli' | 'gui') {
  practiceOpened.value = true
  practiceMode.value = mode; practiceVisible.value = true; practiceMinimized.value = false
}
function closePractice() { practiceVisible.value = false; practiceExpanded.value = false }
function minimizePractice() { practiceMinimized.value = true; practiceExpanded.value = false }
onMounted(() => {
  readerMounted.value = true; recordVisit()
  try { sidebarCollapsed.value = localStorage.getItem('attackbox-reader-outline-collapsed') === 'true' } catch {}
  updateViewport(); window.addEventListener('resize', updateViewport)
})
onBeforeUnmount(() => window.removeEventListener('resize', updateViewport))

const userAnswers = ref<Record<string, string>>({})
const answerFeedbacks = ref<Record<string, { status: 'checking' | 'success' | 'error'; message: string }>>({})
const savedProgress = ref<ChapterProgress>({ completedAt: null, passedQuestionIds: [], isSaved: false })
const completing = ref(false), progressNotice = ref(''), completionError = ref(''), readerMounted = ref(false)
function applySavedProgress(progress: ChapterProgress) {
  // Success records are monotonic: a slower earlier answer must not undo completion.
  savedProgress.value = { ...progress, completedAt: savedProgress.value.completedAt || progress.completedAt,
    passedQuestionIds: [...new Set([...savedProgress.value.passedQuestionIds, ...progress.passedQuestionIds])] }
}
let lastVisitKey = '', chapterGeneration = 0
const allAnswersCorrect = computed(() => !!chapterData.value?.questions.length && chapterData.value.questions.every(q => savedProgress.value.passedQuestionIds.includes(q.id) || answerFeedbacks.value[q.id]?.status === 'success'))
watch(() => chapterResponse.value?.data, data => {
  if (data?.chapter.slug === chapterSlug.value) savedProgress.value = data.progress
}, { immediate: true })
watch([moduleSlug, chapterSlug, learnerId], async () => {
  chapterGeneration++; completing.value = false; userAnswers.value = {}; answerFeedbacks.value = {}; mobileOutlineOpen.value = false
  savedProgress.value = { completedAt: null, passedQuestionIds: [], isSaved: false }; progressNotice.value = ''; completionError.value = ''; lastVisitKey = ''
  await nextTick(); scrollElement.value?.scrollTo({ top: 0 })
})
async function recordVisit() {
  const data = chapterResponse.value?.data
  if (!readerMounted.value || !learnerId.value || chapterError.value || !data || data.chapter.slug !== chapterSlug.value) return
  const key = `${learnerId.value}:${data.chapter.id}`
  if (lastVisitKey === key) return
  lastVisitKey = key
  const generation = chapterGeneration
  try {
    await $fetch(`/api/modules/${moduleSlug.value}/chapters/${chapterSlug.value}/visit`, { method: 'POST' })
    if (generation === chapterGeneration) progressNotice.value = ''
  } catch { if (generation === chapterGeneration) { lastVisitKey = ''; progressNotice.value = '阅读记录暂未保存，请重试。' } }
}
watch([() => chapterResponse.value?.data.chapter.id, learnerId], recordVisit, { flush: 'post' })
async function completeChapter() {
  if (completing.value) return
  const generation = chapterGeneration
  completing.value = true; completionError.value = ''
  try {
    const result = await $fetch<{ data: { progress: ChapterProgress } }>(`/api/modules/${moduleSlug.value}/chapters/${chapterSlug.value}/complete`, { method: 'POST' })
    if (generation !== chapterGeneration) return
    applySavedProgress(result.data.progress)
    await refreshModule()
  } catch (error: any) { if (generation === chapterGeneration) completionError.value = error?.data?.message || '保存失败，请重试。' }
  finally { if (generation === chapterGeneration) completing.value = false }
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

function renderPrompt(markdown: string): string {
  return renderMarkdown(markdown || '').html
}

async function handleCheckAnswer(q: QuestionItem) {
  if (savedProgress.value.completedAt || answerFeedbacks.value[q.id]?.status === 'checking') return
  const generation = chapterGeneration
  const input = userAnswers.value[q.id] || ''
  if (!input.length) {
    answerFeedbacks.value[q.id] = {
      status: 'error',
      message: '请输入你的答案后再进行校验。'
    }
    return
  }

  answerFeedbacks.value[q.id] = { status: 'checking', message: '正在校验…' }
  try {
    const res = await $fetch<{ ok: boolean; data: { correct: boolean; errorHint: string | null; progress: ChapterProgress } }>(
      `/api/modules/${moduleSlug.value}/chapters/${chapterSlug.value}/attempt`,
      { method: 'POST', body: { questionId: q.id, answer: input } }
    )
    if (generation !== chapterGeneration) return
    const result = res.data
    applySavedProgress(result.progress)
    answerFeedbacks.value[q.id] = result.correct
      ? { status: 'success', message: '回答正确' }
      : {
          status: 'error',
          message: result.errorHint || '答案不正确，请再试一次。'
        }
    if (learnerId.value) await refreshModule()
  } catch (error: any) {
    if (generation !== chapterGeneration) return
    answerFeedbacks.value[q.id] = {
      status: 'error',
      message: error?.data?.message || '校验失败,请稍后重试。'
    }
  }
}

useHead({
  title: computed(() => {
    if (chapterData.value && moduleData.value) {
      return `${chapterData.value.title} — ${moduleData.value.title}`
    }
    return '学习章节 — AttackBox'
  })
})
</script>

<template>
  <div class="reading-page">
    <div v-if="chapterError || (!chapterPending && !chapterData)" class="auth-gate-container">
      <section class="auth-gate-card">
        <LockKeyhole v-if="chapterError?.statusCode === 401" :size="28" aria-hidden="true" /><BookOpen v-else :size="28" aria-hidden="true" />
        <h1>{{ chapterError?.statusCode === 401 ? '登录后继续学习' : chapterError?.statusCode === 403 ? '先完成前面的章节' : '章节暂不可用' }}</h1>
        <p>{{ chapterError?.statusCode === 401 ? '当前章节需要登录账号后阅读。' : chapterError?.statusCode === 403 ? '返回模块，继续尚未完成的章节。' : '该章节可能已更新或下架。' }}</p>
        <NuxtLink v-if="chapterError?.statusCode === 401" :to="'/auth?returnTo=' + encodeURIComponent(route.fullPath)" class="gate-btn primary">登录并继续</NuxtLink>
        <NuxtLink :to="`/learn/modules/${moduleSlug}`" class="gate-btn secondary">返回模块</NuxtLink>
      </section>
    </div>
    <div v-else-if="chapterData" class="learn-shell" :class="{ 'outline-collapsed': sidebarCollapsed }">
      <aside class="reading-sidebar" aria-label="模块目录">
        <ChapterOutline :module-slug="moduleSlug" :module-title="moduleData?.title || '学习模块'" :current-slug="chapterSlug" :chapters="moduleData?.chapters || []" :collapsed="sidebarCollapsed" @toggle="toggleSidebar" />
      </aside>
      <div class="reading-main">
        <div class="reading-top-bar">
          <div class="toolbar-left">
            <DialogRoot v-model:open="mobileOutlineOpen">
              <DialogTrigger class="mobile-menu-btn" aria-label="打开模块目录"><PanelLeft :size="16" aria-hidden="true" /></DialogTrigger>
              <DialogPortal>
                <DialogOverlay class="outline-overlay" />
                <DialogContent class="mobile-outline">
                  <DialogTitle class="sr-only">模块目录</DialogTitle><DialogDescription class="sr-only">选择章节，或关闭目录以继续阅读。</DialogDescription>
                  <ChapterOutline :module-slug="moduleSlug" :module-title="moduleData?.title || '学习模块'" :current-slug="chapterSlug" :chapters="moduleData?.chapters || []" @toggle="mobileOutlineOpen = false" @navigate="mobileOutlineOpen = false" />
                </DialogContent>
              </DialogPortal>
            </DialogRoot>
            <NuxtLink :to="`/learn/modules/${moduleSlug}`" class="module-return"><ArrowLeft :size="14" aria-hidden="true" />返回模块</NuxtLink>
          </div>
          <div class="toolbar-tools" role="group" aria-label="练习环境">
            <button type="button" :disabled="!readerMounted" :aria-pressed="practiceVisible && !practiceMinimized && practiceMode === 'cli'" @click="openPractice('cli')"><Terminal :size="15" aria-hidden="true" /><span>CLI</span><span class="tool-word">终端</span></button>
            <button type="button" :disabled="!readerMounted" :aria-pressed="practiceVisible && !practiceMinimized && practiceMode === 'gui'" @click="openPractice('gui')"><Monitor :size="15" aria-hidden="true" /><span>GUI</span><span class="tool-word">桌面</span></button>
          </div>
        </div>
        <div ref="scrollElement" class="reading-scroll" :inert="practiceVisible && !practiceMinimized && practiceExpanded" tabindex="0" aria-label="章节正文">
          <article class="article-body">
            <div class="chapter-meta"><span>第 {{ chapterIndex + 1 }} / {{ moduleData?.chapters.length || 1 }} 章</span><span v-if="chapterData.durationMinutes"><Clock3 :size="13" aria-hidden="true" />约 {{ chapterData.durationMinutes }} 分钟</span></div>
            <h1 class="chapter-main-title">{{ chapterData.title }}</h1>
            <details v-if="chapterResponse?.data.target" class="learning-target"><summary><Box :size="17" aria-hidden="true"/><span>实验环境<span v-if="chapterResponse.data.target.status==='configured'"> · {{chapterResponse.data.target.name}}</span></span><small>{{chapterResponse.data.target.status==='configured'?'环境说明':'暂不可用'}}</small></summary><template v-if="chapterResponse.data.target.status==='configured'"><p class="target-scope">{{chapterResponse.data.target.scope==='module'?'模块默认模板':'本章指定模板'}} · 尚未开放在线启动</p><p v-if="chapterResponse.data.target.description">{{chapterResponse.data.target.description}}</p><p v-if="chapterResponse.data.target.ports.length">容器服务端口：{{chapterResponse.data.target.ports.map(p=>`${p.port}/${p.protocol.toUpperCase()}`).join('、')}}</p><p v-if="chapterResponse.data.target.accessInstructions" class="target-instructions">{{chapterResponse.data.target.accessInstructions}}</p></template><p v-else>关联的实验环境暂不可用，你可以继续阅读并完成章节练习。</p></details>
            <MarkdownView :content="chapterData.bodyMarkdown" />

            <section v-if="chapterData.questions.length" id="quiz" class="quiz-card" aria-labelledby="quiz-title">
              <div class="quiz-head"><h2 id="quiz-title">章节练习</h2><span class="quiz-badge" :class="{ passed: allAnswersCorrect || savedProgress.completedAt }"><CheckCircle2 v-if="allAnswersCorrect || savedProgress.completedAt" :size="14" aria-hidden="true" />{{ savedProgress.completedAt ? '已完成' : allAnswersCorrect ? '全部通过' : `${chapterData.questions.length} 道题` }}</span></div>
              <form v-for="(q, qIndex) in chapterData.questions" :key="q.id" class="question-item" @submit.prevent="handleCheckAnswer(q)">
                <div :id="`question-${q.id}`" class="question-label"><span class="q-num">{{ qIndex + 1 }}.</span><div class="q-prompt" v-html="renderPrompt(q.promptMarkdown)" /></div>
                <div v-if="!savedProgress.completedAt" class="answer-line">
                  <input :id="`answer-${q.id}`" v-model="userAnswers[q.id]" type="text" class="answer-input" placeholder="输入你的答案" maxlength="2000" autocomplete="off" autocapitalize="off" spellcheck="false" :aria-labelledby="`question-${q.id}`" :aria-describedby="`feedback-${q.id}`" :aria-invalid="answerFeedbacks[q.id]?.status === 'error'" :disabled="answerFeedbacks[q.id]?.status === 'checking'" @input="delete answerFeedbacks[q.id]" />
                  <button type="submit" class="answer-check-btn" :disabled="answerFeedbacks[q.id]?.status === 'checking'"><LoaderCircle v-if="answerFeedbacks[q.id]?.status === 'checking'" :size="14" class="spinning" aria-hidden="true" />{{ answerFeedbacks[q.id]?.status === 'checking' ? '校验中' : '校验答案' }}</button>
                </div>
                <div :id="`feedback-${q.id}`" class="answer-feedback" :class="savedProgress.completedAt ? 'success' : answerFeedbacks[q.id]?.status || (savedProgress.passedQuestionIds.includes(q.id) ? 'success' : undefined)" role="status" aria-live="polite"><template v-if="savedProgress.completedAt"><CheckCircle2 :size="14" aria-hidden="true" />已完成</template><template v-else-if="answerFeedbacks[q.id]"><CheckCircle2 v-if="answerFeedbacks[q.id].status === 'success'" :size="14" aria-hidden="true" /><CircleAlert v-else-if="answerFeedbacks[q.id].status === 'error'" :size="14" aria-hidden="true" />{{ answerFeedbacks[q.id].message }}</template><template v-else-if="savedProgress.passedQuestionIds.includes(q.id)"><CheckCircle2 :size="14" aria-hidden="true" />已通过</template></div>
              </form>
            </section>

            <div v-if="!chapterData.questions.length" class="chapter-completion">
              <span v-if="savedProgress.completedAt" class="completion-saved"><CheckCircle2 :size="16" aria-hidden="true" />本章已完成</span>
              <button v-else-if="learnerId" type="button" class="complete-chapter-btn" :disabled="completing" @click="completeChapter">{{ completing ? '正在保存…' : '完成本章' }}</button>
              <NuxtLink v-else :to="'/auth?returnTo=' + encodeURIComponent(route.fullPath)" class="completion-login">登录后保存学习进度</NuxtLink>
            </div>
            <p v-else-if="!learnerId && allAnswersCorrect" class="guest-progress">本次练习已通过。<NuxtLink :to="'/auth?returnTo=' + encodeURIComponent(route.fullPath)">登录后可保存学习进度</NuxtLink></p>
            <p v-if="completionError" class="progress-notice" role="alert">{{ completionError }} <button type="button" @click="completeChapter">重试保存</button></p>
            <p v-if="progressNotice" class="progress-notice" role="alert">{{ progressNotice }} <button type="button" @click="recordVisit">重试记录</button></p>
            <nav class="next-row" aria-label="章节翻页">
              <NuxtLink v-if="prevChapter && prevChapter.lockReason !== 'prerequisite'" :to="prevChapter.lockReason === 'login' ? `/auth?returnTo=${encodeURIComponent(`/learn/modules/${moduleSlug}/${prevChapter.slug}`)}` : `/learn/modules/${moduleSlug}/${prevChapter.slug}`" class="btn-nav prev-btn"><ArrowLeft :size="15" aria-hidden="true" /><span>上一章<span class="nav-title">{{ prevChapter.title }}</span></span></NuxtLink><span v-else />
              <button v-if="nextChapter?.lockReason === 'prerequisite'" type="button" class="btn-nav next-locked" disabled>完成前面的章节后继续</button><NuxtLink v-else-if="nextChapter" :to="nextChapter.lockReason === 'login' ? `/auth?returnTo=${encodeURIComponent(`/learn/modules/${moduleSlug}/${nextChapter.slug}`)}` : `/learn/modules/${moduleSlug}/${nextChapter.slug}`" class="btn-nav next-btn"><span>{{ nextChapter.lockReason === 'login' ? '登录后继续' : '下一章' }}<span class="nav-title">{{ nextChapter.title }}</span></span><LockKeyhole v-if="nextChapter.isLocked" :size="15" aria-hidden="true" /><ArrowRight v-else :size="15" aria-hidden="true" /></NuxtLink><span v-else class="last-chapter">本模块最后一章</span>
            </nav>
          </article>
        </div>
        <PracticeDock v-if="practiceOpened" v-show="practiceVisible && !practiceMinimized" :key="learnerId" :active="practiceVisible && !practiceMinimized" v-model:height="dockHeight" v-model:expanded="practiceExpanded" :mode="practiceMode" :max-height="maxDockHeight" @minimize="minimizePractice" @close="closePractice" />
        <button v-if="practiceVisible && practiceMinimized" type="button" class="dock-restore" aria-label="恢复练习面板" @click="practiceMinimized = false"><Terminal v-if="practiceMode === 'cli'" :size="14" aria-hidden="true" /><Monitor v-else :size="14" aria-hidden="true" />{{ practiceMode === 'cli' ? '终端' : '桌面' }}<ChevronUp :size="15" aria-hidden="true" /></button>
      </div>
    </div>
    <div v-else class="reading-loading" role="status">正在加载章节…</div>
  </div>
</template>

<style scoped>
.chapter-completion { margin-top: 32px; display: flex; justify-content: flex-end; }
.complete-chapter-btn { border: 0; border-radius: 6px; padding: 11px 18px; background: var(--green); color: var(--on-accent); font-size: 12px; cursor: pointer; }
.complete-chapter-btn:disabled { opacity: .6; cursor: wait; }
.completion-saved { display: inline-flex; align-items: center; gap: 7px; font-size: 12px; color: var(--success-ink); }
.completion-login, .guest-progress, .progress-notice { font-size: 12px; color: var(--muted); margin-top: 18px; }
.guest-progress a, .progress-notice button { color: var(--green); text-decoration: underline; }
.progress-notice button { border: 0; background: transparent; cursor: pointer; }
.next-locked { border: 0; background: transparent; cursor: not-allowed; text-align: right; }

.reading-page { background: var(--reader-bg); color: var(--ink); }
.learn-shell { display: grid; grid-template-columns: 248px minmax(0, 1fr); height: calc(100dvh - 68px); min-height: 320px; }
.learn-shell.outline-collapsed { grid-template-columns: 56px minmax(0, 1fr); }
.reading-sidebar { min-height: 0; background: var(--paper); border-right: 1px solid var(--line); }
.reading-main { min-width: 0; min-height: 0; display: flex; flex-direction: column; position: relative; }
.reading-top-bar { display: flex; align-items: center; justify-content: space-between; gap: 16px; height: 45px; flex-shrink: 0; padding: 0 24px; border-bottom: 1px solid var(--line); background: var(--reader-bg); }
.toolbar-left, .toolbar-tools { display: flex; align-items: center; gap: 12px; }
.module-return { display: inline-flex; align-items: center; gap: 7px; font-size: 11px; text-decoration: none; color: var(--muted); }
.module-return:hover { color: var(--ink); }
.toolbar-tools { gap: 5px; }
.toolbar-tools button { display: inline-flex; align-items: center; gap: 6px; padding: 6px 9px; border: 1px solid transparent; border-radius: 5px; background: transparent; font-size: 11px; color: var(--muted); cursor: pointer; }
.toolbar-tools button:hover { background: var(--soft); color: var(--ink); }
.toolbar-tools button[aria-pressed='true'] { border-color: var(--line); background: var(--paper); color: var(--ink); }
.reading-scroll { flex: 1; overflow: auto; min-height: 0; overscroll-behavior: contain; scroll-padding-top: 24px; }
.article-body { width: 100%; min-width: 0; padding: 36px clamp(32px, 4vw, 72px) 64px; overflow-wrap: anywhere; }
.chapter-meta { display: flex; flex-wrap: wrap; gap: 16px; font-size: 11px; color: var(--muted); margin-bottom: 12px; }
.chapter-meta > span { display: inline-flex; align-items: center; gap: 5px; }
.chapter-main-title { font-size: clamp(26px, 2.7vw, 32px); font-weight: 700; line-height: 1.4; letter-spacing: -.6px; margin: 0 0 20px; overflow-wrap: anywhere; }
.article-body :deep(.markdown-body) { color: var(--reader-ink); font-size: 17px; line-height: 1.9; }
.article-body :deep(.markdown-body p) { color: var(--reader-ink); opacity: 1; }
.article-body :deep(.markdown-body h2) { font-size: 22px; margin-top: 1.6em; border: 0; padding-bottom: 0; color: var(--ink); }
.article-body :deep(.markdown-body h3) { color: var(--ink); font-size: 19px; }
.article-body :deep(.markdown-body pre) { font-size: 14px; }
.article-body :deep(.markdown-body .callout p) { font-size: inherit; }
.article-body :deep(.markdown-body a) { color: var(--green); }
.article-body :deep(.markdown-body :not(pre) > code) { color: var(--ink); }
.article-body :deep(.markdown-body blockquote) { border-left-color: var(--focus); background: var(--soft); }
.article-body :deep(.markdown-body table) { display: block; max-width: 100%; overflow-x: auto; }
.article-body :deep(.markdown-body img) { max-width: 100%; height: auto; }
.quiz-card { width: 100%; margin-top: 44px; padding-top: 30px; border-top: 1px solid var(--line); }
.quiz-head { display: flex; justify-content: space-between; align-items: center; gap: 14px; margin-bottom: 22px; }
.quiz-head h2 { font-size: 22px; font-weight: 650; margin: 0; letter-spacing: -.4px; }
.quiz-badge { display: inline-flex; align-items: center; gap: 5px; font-size: 11px; color: var(--muted); }
.quiz-badge.passed { color: var(--success-ink); }
.question-item { width: 100%; margin: 0; }
.question-item + .question-item { padding-top: 24px; margin-top: 20px; border-top: 1px solid var(--line); }
.question-label { display: flex; align-items: baseline; gap: 8px; font-size: 17px; line-height: 1.9; color: var(--ink); margin-bottom: 12px; }
.q-num { flex-shrink: 0; color: var(--muted); font-size: 12px; font-family: monospace; }
.q-prompt { min-width: 0; overflow-wrap: anywhere; }
.q-prompt :deep(p) { margin: 0; }
.q-prompt :deep(pre) { overflow-x: auto; }
.answer-line { display: flex; align-items: stretch; gap: 10px; width: 100%; }
.answer-input { flex: 1 1 0; width: 0; min-width: 0; padding: 11px 13px; border: 1px solid var(--line); border-radius: 6px; background: var(--paper); color: var(--ink); font: 13px/1.6 monospace; }
.answer-input[aria-invalid='true'] { border-color: var(--error-ink); }
.answer-check-btn { display: inline-flex; align-items: center; justify-content: center; gap: 6px; flex-shrink: 0; padding: 10px 16px; border: 1px solid var(--line); border-radius: 6px; background: var(--paper); color: var(--ink); font-size: 12px; cursor: pointer; }
.answer-check-btn:hover:not(:disabled) { background: var(--soft); }
.answer-check-btn:disabled, .answer-input:disabled { opacity: .65; cursor: wait; }
.answer-feedback { display: flex; align-items: baseline; gap: 6px; font-size: 12px; line-height: 1.7; min-height: 20px; margin-top: 8px; }
.answer-feedback svg { align-self: center; flex-shrink: 0; }
.answer-feedback.success { color: var(--success-ink); }
.answer-feedback.error { color: var(--error-ink); }
.answer-feedback.checking { color: var(--muted); }
.next-row { display: flex; justify-content: space-between; align-items: center; gap: 24px; margin-top: 36px; padding-top: 24px; border-top: 1px solid var(--line); }
.btn-nav { display: inline-flex; align-items: center; gap: 12px; min-width: 0; max-width: 48%; padding: 8px 0; font-size: 11px; line-height: 1.7; color: var(--muted); text-decoration: none; }
.btn-nav:hover { color: var(--green); }
.nav-title { display: block; color: var(--ink); font-size: 12px; overflow-wrap: anywhere; }
.next-btn { text-align: right; }
.last-chapter { color: var(--muted); font-size: 11px; }
.dock-restore { display: flex; align-items: center; justify-content: center; gap: 8px; min-height: 35px; flex-shrink: 0; background: var(--paper); color: var(--muted); border: 0; border-top: 1px solid var(--line); cursor: pointer; font-size: 11px; }
.mobile-menu-btn { display: none; }
.auth-gate-container, .reading-loading { display: grid; place-items: center; min-height: calc(100dvh - 68px); padding: 24px; color: var(--muted); font-size: 13px; }
.auth-gate-card { width: 100%; max-width: 420px; padding: 36px; border: 1px solid var(--line); border-radius: 12px; text-align: center; background: var(--paper); }
.auth-gate-card > svg { margin: 0 auto 18px; color: var(--muted); }
.auth-gate-card h1 { font-size: 21px; font-weight: 650; margin: 0 0 10px; color: var(--ink); }
.auth-gate-card p { font-size: 13px; line-height: 1.8; margin: 0 0 24px; }
.gate-btn { display: block; padding: 10px 14px; font-size: 12px; border-radius: 6px; text-decoration: none; }
.gate-btn.primary { background: var(--green); color: var(--on-accent); margin-bottom: 10px; }
.gate-btn.secondary { border: 1px solid var(--line); color: var(--ink); }
svg { flex-shrink: 0; }
a:focus-visible, button:focus-visible, input:focus-visible { outline: 2px solid var(--focus); outline-offset: 3px; }
.reading-scroll:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
.spinning { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 900px) {
  .learn-shell, .learn-shell.outline-collapsed { grid-template-columns: minmax(0, 1fr); }
  .reading-sidebar { display: none; }
  .mobile-menu-btn { display: grid; place-items: center; padding: 5px; border: 0; border-radius: 5px; background: transparent; color: var(--muted); cursor: pointer; }
  .article-body { padding: 30px 32px 48px; }
}
@media (max-width: 600px) {
  .reading-top-bar { padding: 0 14px; gap: 10px; }
  .toolbar-left { gap: 8px; }
  .tool-word { display: none; }
  .article-body { padding: 26px 20px 40px; }
  .answer-check-btn { padding: 10px 12px; }
  .next-row { gap: 16px; }
}
@media (prefers-reduced-motion: reduce) { .spinning { animation: none; } }
</style>

<style>
/* Portalled mobile outline stays in the platform theme and traps focus. */
.outline-overlay { position: fixed; inset: 0; background: #0006; z-index: 80; }
.mobile-outline { position: fixed; top: 0; left: 0; bottom: 0; width: min(300px, 85vw); background: var(--paper); color: var(--ink); border-right: 1px solid var(--line); z-index: 81; }
</style>

<style scoped>.learning-target{border:1px solid var(--line);border-radius:8px;background:var(--paper);padding:16px 20px;margin:0 0 28px;font-size:13px}.learning-target summary{display:flex;align-items:center;gap:10px;cursor:pointer;list-style:none}.learning-target summary>span{flex:1;font-weight:600}.learning-target small,.target-scope{color:var(--muted);font-size:12px}.learning-target p{font-size:13px;line-height:1.8;margin:14px 0 0}.target-instructions{white-space:pre-wrap;overflow-wrap:anywhere}</style>
