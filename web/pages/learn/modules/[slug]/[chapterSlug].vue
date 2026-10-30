<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useFetch, useHead } from '#imports'
import {
  ArrowRight,
  LockKeyhole,
  Clock3,
  Terminal,
  Monitor,
  ChevronUp,
  CheckCircle2,
  CircleAlert,
  BookOpen
} from 'lucide-vue-next'
import ReaderLayout from '~/components/reader/Layout.vue'
import ReaderArticle from '~/components/reader/Article.vue'
import ChapterOutline from '~/components/learning/ChapterOutline.vue'
import PracticeDock from '~/components/learning/PracticeDock.vue'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import { useAuth } from '~/composables/useAuth'
import type { LearningTarget } from '~/shared/learning-target'
import type { ChapterProgress, LearningChapter } from '~/shared/learning'
import { renderMarkdown, type MarkdownHeading } from '~/utils/markdown'
import { apiErrorMessage, apiFetch } from '~/utils/api'

definePageMeta({
  layout: 'learning'
})

interface QuestionItem {
  id: string
  position: number
  promptMarkdown: string
  promptHtml?: string
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
  data: {
    imageUrls?: Record<string, string>
    chapter: ChapterInfo
    questions: QuestionItem[]
    progress: ChapterProgress
    target: LearningTarget
  }
}>(() => `/api/modules/${moduleSlug.value}/chapters/${chapterSlug.value}`, {
  key: `learning:chapter:${moduleSlug.value}:${chapterSlug.value}`,
  watch: [learnerId]
})

// 2. 获取所属模块完整大纲
const { data: moduleResponse, refresh: refreshModule } = await useFetch<{
  ok: boolean
  data: { module: Omit<ModuleOutline, 'chapters'>; chapters: ModuleOutlineChapter[] }
}>(() => `/api/modules/${moduleSlug.value}`, {
  key: `learning:outline:${moduleSlug.value}`,
  watch: [learnerId]
})

function renderPrompt(markdown: string, imageUrls = chapterResponse.value?.data.imageUrls): string {
  return renderMarkdown(markdown || '', imageUrls, { copyCode: true }).html
}

const chapterData = computed<ChapterDetail | null>(() => {
  const data = chapterResponse.value?.data
  if (!data) return null
  return {
    title: data.chapter.title,
    bodyMarkdown: data.chapter.bodyMarkdown,
    durationMinutes: data.chapter.durationMinutes,
    questions: (data.questions || []).map(q => ({
      ...q,
      promptHtml: renderPrompt(q.promptMarkdown, data.imageUrls)
    }))
  }
})

const moduleData = computed<ModuleOutline | null>(() => {
  const data = moduleResponse.value?.data
  if (!data) return null
  return { ...data.module, chapters: data.chapters }
})

const readerLayout = ref<InstanceType<typeof ReaderLayout> | null>(null)
const readerArticle = ref<InstanceType<typeof ReaderArticle> | null>(null)
const scrollElement = computed(() => readerLayout.value?.scrollElement ?? null)
const articleElement = computed(() => readerArticle.value?.element ?? null)
const sections = ref<MarkdownHeading[]>([])
const readerIdentity = computed(() =>
  chapterResponse.value?.data.chapter.id
    ? `${learnerId.value || 'guest'}:${chapterResponse.value.data.chapter.id}`
    : ''
)
const readerContent = computed(() => chapterData.value?.bodyMarkdown || '')
const { activeSection, goToSection } = useReaderNavigation(
  scrollElement,
  articleElement,
  readerIdentity,
  readerContent,
  sections
)
function headingsLoaded(headings: MarkdownHeading[]) {
  sections.value = headings.filter(heading => heading.level >= 2)
}
const { sandbox: readerSandbox } = useSandboxSession()
const environmentReady = computed(
  () => readerSandbox.value.instance?.state === 'running' && readerSandbox.value.instance.connected
)
const practiceMode = ref<'cli' | 'gui'>('cli')
const practiceOpened = ref(false)
const practiceVisible = ref(false),
  practiceMinimized = ref(false)
const workspace = computed(() => readerLayout.value?.workspaceElement ?? null)
const dock = usePracticeLayout(workspace)
watch(environmentReady, ready => {
  if (!ready) {
    practiceOpened.value = false
    practiceVisible.value = false
    practiceMinimized.value = false
  }
})
function openPractice(mode: 'cli' | 'gui') {
  if (!environmentReady.value) return
  practiceOpened.value = true
  practiceMode.value = mode
  practiceVisible.value = true
  practiceMinimized.value = false
}
function minimizePractice() {
  practiceMinimized.value = true
}
onMounted(() => {
  readerMounted.value = true
  recordVisit()
})

const userAnswers = ref<Record<string, string>>({})
const answerFeedbacks = ref<
  Record<string, { status: 'checking' | 'success' | 'error'; message: string }>
>({})
const savedProgress = ref<ChapterProgress>({
  completedAt: null,
  passedQuestionIds: [],
  isSaved: false
})
const celebrationOpen = ref(false)
const moduleCompleted = computed(
  () => !!moduleData.value?.chapters.length && moduleData.value.chapters.every(c => !!c.completedAt)
)
function celebrateCompletion(wasCompleted: boolean, generation: number) {
  if (generation === chapterGeneration && !wasCompleted && moduleCompleted.value)
    celebrationOpen.value = true
}
const completing = ref(false),
  progressNotice = ref(''),
  completionError = ref(''),
  readerMounted = ref(false)
function applySavedProgress(progress: ChapterProgress) {
  // Success records are monotonic: a slower earlier answer must not undo completion.
  savedProgress.value = {
    ...progress,
    completedAt: savedProgress.value.completedAt || progress.completedAt,
    passedQuestionIds: [
      ...new Set([...savedProgress.value.passedQuestionIds, ...progress.passedQuestionIds])
    ]
  }
}
let lastVisitKey = '',
  chapterGeneration = 0
const allAnswersCorrect = computed(
  () =>
    !!chapterData.value?.questions.length &&
    chapterData.value.questions.every(
      q =>
        savedProgress.value.passedQuestionIds.includes(q.id) ||
        answerFeedbacks.value[q.id]?.status === 'success'
    )
)
watch(
  () => chapterResponse.value?.data,
  data => {
    if (data?.chapter.slug === chapterSlug.value) savedProgress.value = data.progress
  },
  { immediate: true }
)
watch([moduleSlug, chapterSlug, learnerId], () => {
  chapterGeneration++
  celebrationOpen.value = false
  completing.value = false
  userAnswers.value = {}
  answerFeedbacks.value = {}
  readerLayout.value?.closeOutline()
  savedProgress.value = { completedAt: null, passedQuestionIds: [], isSaved: false }
  progressNotice.value = ''
  completionError.value = ''
  lastVisitKey = ''
})
async function recordVisit() {
  const data = chapterResponse.value?.data
  if (
    !readerMounted.value ||
    !learnerId.value ||
    chapterError.value ||
    !data ||
    data.chapter.slug !== chapterSlug.value
  )
    return
  const key = `${learnerId.value}:${data.chapter.id}`
  if (lastVisitKey === key) return
  lastVisitKey = key
  const generation = chapterGeneration
  try {
    await apiFetch(`/api/modules/${moduleSlug.value}/chapters/${chapterSlug.value}/visit`, {
      method: 'POST'
    })
    if (generation === chapterGeneration) progressNotice.value = ''
  } catch {
    if (generation === chapterGeneration) {
      lastVisitKey = ''
      progressNotice.value = '阅读记录暂未保存，请重试。'
    }
  }
}
watch([() => chapterResponse.value?.data.chapter.id, learnerId], recordVisit, { flush: 'post' })
async function completeChapter() {
  if (completing.value) return
  const generation = chapterGeneration
  const wasModuleCompleted = moduleCompleted.value
  completing.value = true
  completionError.value = ''
  try {
    const result = await apiFetch<{ data: { progress: ChapterProgress } }>(
      `/api/modules/${moduleSlug.value}/chapters/${chapterSlug.value}/complete`,
      { method: 'POST' }
    )
    if (generation !== chapterGeneration) return
    applySavedProgress(result.data.progress)
    await refreshModule()
    celebrateCompletion(wasModuleCompleted, generation)
  } catch (error) {
    if (generation === chapterGeneration)
      completionError.value = apiErrorMessage(error, '保存失败，请重试。')
  } finally {
    if (generation === chapterGeneration) completing.value = false
  }
}

const chapterIndex = computed(() => {
  if (!moduleData.value || !chapterData.value) return 0
  const idx = moduleData.value.chapters.findIndex(c => c.slug === chapterSlug.value)
  return idx >= 0 ? idx : 0
})

const nextChapter = computed(() => {
  if (!moduleData.value) return null
  const idx = chapterIndex.value
  const chapters = moduleData.value.chapters
  return idx < chapters.length - 1 ? chapters[idx + 1] : null
})

async function handleCheckAnswer(q: QuestionItem) {
  if (savedProgress.value.completedAt || answerFeedbacks.value[q.id]?.status === 'checking') return
  const generation = chapterGeneration
  const wasModuleCompleted = moduleCompleted.value
  const input = userAnswers.value[q.id] || ''
  if (!input.length) {
    answerFeedbacks.value[q.id] = {
      status: 'error',
      message: '请输入你的答案后再进行校验。'
    }
    return
  }

  answerFeedbacks.value[q.id] = { status: 'checking', message: '' }
  try {
    const res = await apiFetch<{
      ok: boolean
      data: { correct: boolean; errorHint: string | null; progress: ChapterProgress }
    }>(`/api/modules/${moduleSlug.value}/chapters/${chapterSlug.value}/attempt`, {
      method: 'POST',
      body: { questionId: q.id, answer: input }
    })
    if (generation !== chapterGeneration) return
    const result = res.data
    applySavedProgress(result.progress)
    answerFeedbacks.value[q.id] = result.correct
      ? { status: 'success', message: '回答正确' }
      : {
          status: 'error',
          message: result.errorHint || '答案不正确，请再试一次。'
        }
    if (learnerId.value) {
      await refreshModule()
      celebrateCompletion(wasModuleCompleted, generation)
    }
  } catch (error) {
    if (generation !== chapterGeneration) return
    answerFeedbacks.value[q.id] = {
      status: 'error',
      message: apiErrorMessage(error, '校验失败,请稍后重试。')
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
        <LockKeyhole
          v-if="chapterError?.statusCode === 401"
          :size="28"
          aria-hidden="true"
        /><BookOpen v-else :size="28" aria-hidden="true" />
        <h1>
          {{
            chapterError?.statusCode === 401
              ? '登录后继续学习'
              : chapterError?.statusCode === 403
                ? '先完成前面的章节'
                : '章节暂不可用'
          }}
        </h1>
        <p>
          {{
            chapterError?.statusCode === 401
              ? '当前章节需要登录账号后阅读。'
              : chapterError?.statusCode === 403
                ? '返回模块，继续尚未完成的章节。'
                : '该章节可能已更新或下架。'
          }}
        </p>
        <NuxtLink
          v-if="chapterError?.statusCode === 401"
          :to="'/auth?returnTo=' + encodeURIComponent(route.fullPath)"
          class="gate-btn primary"
          >登录并继续</NuxtLink
        >
        <NuxtLink :to="`/learn/modules/${moduleSlug}`" class="gate-btn secondary"
          >返回模块</NuxtLink
        >
      </section>
    </div>
    <ReaderLayout
      v-else-if="chapterData"
      ref="readerLayout"
      outline-label="模块目录"
      :outline-title="moduleData?.title || '学习模块'"
      scroll-label="章节正文"
      :back-to="`/learn/modules/${moduleSlug}`"
      back-label="返回模块"
      :side-practice="practiceVisible && !practiceMinimized && dock.placement.value === 'right'"
    >
      <template #outline="{ collapsed, close }">
        <ChapterOutline
          :module-slug="moduleSlug"
          :current-slug="chapterSlug"
          :chapters="moduleData?.chapters || []"
          :sections="sections"
          :active-section="activeSection"
          :collapsed="collapsed"
          @section="goToSection"
          @navigate="close"
        />
      </template>
      <template #tools>
        <LearningEnvironmentStatus />
        <button
          v-if="environmentReady"
          class="tool-launch"
          type="button"
          :disabled="!readerMounted"
          :aria-pressed="practiceVisible && !practiceMinimized && practiceMode === 'cli'"
          @click="openPractice('cli')"
        >
          <Terminal :size="15" aria-hidden="true" /><span>CLI</span
          ><span class="tool-word">终端</span>
        </button>
        <button
          v-if="environmentReady"
          class="tool-launch"
          type="button"
          :disabled="!readerMounted"
          :aria-pressed="practiceVisible && !practiceMinimized && practiceMode === 'gui'"
          @click="openPractice('gui')"
        >
          <Monitor :size="15" aria-hidden="true" /><span>GUI</span
          ><span class="tool-word">桌面</span>
        </button>
      </template>
      <ReaderArticle ref="readerArticle">
        <div class="chapter-meta">
          <span>第 {{ chapterIndex + 1 }} / {{ moduleData?.chapters.length || 1 }} 章</span
          ><span v-if="chapterData.durationMinutes"
            ><Clock3 :size="13" aria-hidden="true" />约 {{ chapterData.durationMinutes }} 分钟</span
          >
        </div>
        <h1 class="chapter-main-title">{{ chapterData.title }}</h1>
        <MarkdownView
          :content="chapterData.bodyMarkdown"
          :copyable="true"
          :image-urls="chapterResponse?.data.imageUrls"
          @headings-loaded="headingsLoaded"
        />
        <TargetEnvironment
          v-if="chapterResponse?.data.target"
          :key="chapterResponse.data.chapter.id"
          :target="chapterResponse.data.target"
          :context="{
            kind: 'chapter',
            moduleSlug: String(route.params.slug),
            chapterSlug: String(route.params.chapterSlug)
          }"
        />

        <section
          v-if="chapterData.questions.length"
          id="quiz"
          class="quiz-card"
          aria-labelledby="quiz-title"
        >
          <div class="quiz-head">
            <h2 id="quiz-title">章节练习</h2>
            <span
              class="quiz-badge"
              :class="{ passed: allAnswersCorrect || savedProgress.completedAt }"
              ><CheckCircle2
                v-if="allAnswersCorrect || savedProgress.completedAt"
                :size="14"
                aria-hidden="true"
              />{{
                savedProgress.completedAt
                  ? '已完成'
                  : allAnswersCorrect
                    ? '全部通过'
                    : `${chapterData.questions.length} 道题`
              }}</span
            >
          </div>
          <form
            v-for="(q, qIndex) in chapterData.questions"
            :key="q.id"
            class="question-item"
            @submit.prevent="handleCheckAnswer(q)"
          >
            <div :id="`question-${q.id}`" class="question-label">
              <span class="q-num">{{ qIndex + 1 }}.</span>
              <div class="q-prompt" v-html="q.promptHtml" />
            </div>
            <div v-if="!savedProgress.completedAt" class="answer-line">
              <UiInput
                :id="`answer-${q.id}`"
                v-model="userAnswers[q.id]"
                bare
                type="text"
                class="answer-input"
                placeholder="输入你的答案"
                maxlength="2000"
                autocomplete="off"
                autocapitalize="off"
                spellcheck="false"
                :aria-labelledby="`question-${q.id}`"
                :aria-describedby="`feedback-${q.id}`"
                :aria-invalid="answerFeedbacks[q.id]?.status === 'error'"
                :disabled="answerFeedbacks[q.id]?.status === 'checking'"
                @input="delete answerFeedbacks[q.id]"
              />
              <UiButton
                type="submit"
                variant="secondary"
                class="answer-check-btn"
                :loading="answerFeedbacks[q.id]?.status === 'checking'"
                >校验答案</UiButton
              >
            </div>
            <div
              :id="`feedback-${q.id}`"
              class="answer-feedback"
              :class="
                savedProgress.completedAt
                  ? 'success'
                  : answerFeedbacks[q.id]?.status ||
                    (savedProgress.passedQuestionIds.includes(q.id) ? 'success' : undefined)
              "
              role="status"
              aria-live="polite"
            >
              <template v-if="savedProgress.completedAt"
                ><CheckCircle2 :size="14" aria-hidden="true" />已完成</template
              ><template v-else-if="answerFeedbacks[q.id]"
                ><CheckCircle2
                  v-if="answerFeedbacks[q.id].status === 'success'"
                  :size="14"
                  aria-hidden="true"
                /><CircleAlert
                  v-else-if="answerFeedbacks[q.id].status === 'error'"
                  :size="14"
                  aria-hidden="true"
                />{{ answerFeedbacks[q.id].message }}</template
              ><template v-else-if="savedProgress.passedQuestionIds.includes(q.id)"
                ><CheckCircle2 :size="14" aria-hidden="true" />已通过</template
              >
            </div>
          </form>
        </section>

        <p v-if="completionError" class="progress-notice" role="alert">
          {{ completionError }}
        </p>
        <p v-if="progressNotice" class="progress-notice" role="alert">
          {{ progressNotice }} <button type="button" @click="recordVisit">重试记录</button>
        </p>
        <footer class="chapter-action-row" aria-label="章节进度">
          <template v-if="!savedProgress.completedAt">
            <NuxtLink
              v-if="!learnerId"
              :to="'/auth?returnTo=' + encodeURIComponent(route.fullPath)"
              class="chapter-primary"
              >{{ allAnswersCorrect ? '练习已通过 · 登录后保存进度' : '登录后保存进度' }}</NuxtLink
            >
            <template v-else>
              <span
                v-if="chapterData.questions.length && !allAnswersCorrect"
                class="completion-hint"
                >通过全部练习后完成本章</span
              >
              <button
                type="button"
                class="chapter-primary"
                :disabled="completing || (!!chapterData.questions.length && !allAnswersCorrect)"
                @click="completeChapter"
              >
                <UiBusyLabel :busy="completing"
                  ><CheckCircle2 :size="16" aria-hidden="true" />完成本章</UiBusyLabel
                >
              </button>
            </template>
          </template>
          <template v-else>
            <span class="completion-saved"
              ><CheckCircle2 :size="16" aria-hidden="true" />已完成</span
            >
            <NuxtLink
              v-if="nextChapter && !nextChapter.isLocked"
              :to="`/learn/modules/${moduleSlug}/${nextChapter.slug}`"
              class="chapter-primary"
              >{{ nextChapter.title }}<ArrowRight :size="16" aria-hidden="true"
            /></NuxtLink>
            <NuxtLink
              v-else-if="!nextChapter"
              :to="`/learn/modules/${moduleSlug}`"
              class="chapter-primary"
              >返回模块<ArrowRight :size="16" aria-hidden="true"
            /></NuxtLink>
          </template>
        </footer>
      </ReaderArticle>
      <template #practice>
        <PracticeDock
          v-if="practiceOpened"
          v-show="practiceVisible && !practiceMinimized"
          :key="learnerId"
          :active="practiceVisible && !practiceMinimized"
          :height="dock.height.value"
          :width="dock.width.value"
          :placement="dock.placement.value"
          :side-available="dock.sideAvailable.value"
          :min-width="dock.minWidth"
          :max-width="dock.maxWidth.value"
          :min-height="dock.minHeight.value"
          :max-height="dock.maxHeight.value"
          :mode="practiceMode"
          @update:height="dock.setHeight"
          @update:width="dock.setWidth"
          @update:placement="dock.setPlacement"
          @resize-end="dock.persist"
          @minimize="minimizePractice"
        />
        <button
          v-if="practiceVisible && practiceMinimized"
          type="button"
          class="dock-restore"
          aria-label="恢复练习面板"
          @click="practiceMinimized = false"
        >
          <Terminal v-if="practiceMode === 'cli'" :size="14" aria-hidden="true" /><Monitor
            v-else
            :size="14"
            aria-hidden="true"
          />{{ practiceMode === 'cli' ? '终端' : '桌面'
          }}<ChevronUp :size="15" aria-hidden="true" />
        </button>
      </template>
    </ReaderLayout>
    <UiBusyIndicator v-else class="reading-loading" label="章节加载" />
    <LearningModuleCompletion
      v-model:open="celebrationOpen"
      :module-title="moduleData?.title || '学习模块'"
      :module-slug="moduleSlug"
      :chapter-count="moduleData?.chapters.length || 0"
    />
  </div>
</template>

<style scoped>
.chapter-action-row {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  margin-top: 36px;
  padding-top: 24px;
  border-top: 1px solid var(--line);
}
.chapter-primary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  max-width: 100%;
  min-height: 42px;
  padding: 10px 20px;
  border: 0;
  border-radius: 8px;
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  box-shadow: var(--btn-primary-shadow);
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
  transition:
    transform 0.16s ease,
    background 0.16s ease;
}
.chapter-primary:hover:not(:disabled) {
  transform: translateY(-1px);
  background: var(--btn-primary-hover);
}
.chapter-primary:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.completion-hint,
.progress-notice {
  font-size: 12px;
  color: var(--muted);
}
.completion-saved {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-right: auto;
  color: var(--success-ink);
  font-size: 12px;
}
.progress-notice {
  margin-top: 18px;
}
.progress-notice button {
  background: transparent;
  border: 0;
  color: var(--green);
  cursor: pointer;
}

.reading-page {
  background: var(--reader-bg);
  color: var(--ink);
}

.tool-launch {
  height: 32px;
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  gap: 6px;
  padding: 6px 9px;
  border: 1px solid transparent;
  border-radius: 5px;
  background: transparent;
  font-size: 11px;
  color: var(--muted);
  cursor: pointer;
}
.tool-launch:hover {
  background: var(--soft);
  color: var(--ink);
}
.tool-launch[aria-pressed='true'] {
  border-color: var(--line);
  background: var(--paper);
  color: var(--ink);
}
.chapter-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  font-size: 11px;
  color: var(--muted);
  margin-bottom: 12px;
}
.chapter-meta > span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.chapter-main-title {
  font-size: clamp(26px, 2.7vw, 32px);
  font-weight: 700;
  line-height: 1.4;
  letter-spacing: -0.6px;
  margin: 0 0 20px;
  overflow-wrap: anywhere;
}
.quiz-card {
  width: 100%;
  margin-top: 44px;
  padding-top: 30px;
  border-top: 1px solid var(--line);
}
.quiz-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 14px;
  margin-bottom: 22px;
}
.quiz-head h2 {
  font-size: 22px;
  font-weight: 650;
  margin: 0;
  letter-spacing: -0.4px;
}
.quiz-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: var(--muted);
}
.quiz-badge.passed {
  color: var(--success-ink);
}
.question-item {
  width: 100%;
  margin: 0;
}
.question-item + .question-item {
  padding-top: 24px;
  margin-top: 20px;
  border-top: 1px solid var(--line);
}
.question-label {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 17px;
  line-height: 1.9;
  color: var(--ink);
  margin-bottom: 12px;
}
.q-num {
  flex-shrink: 0;
  color: var(--muted);
  font-size: 12px;
  font-family: monospace;
}
.q-prompt {
  min-width: 0;
  overflow-wrap: anywhere;
}
.q-prompt :deep(p) {
  margin: 0;
}
.q-prompt :deep(pre) {
  overflow-x: auto;
}
.answer-line {
  display: flex;
  align-items: stretch;
  gap: 10px;
  width: 100%;
}
.answer-input {
  flex: 1 1 0;
  width: 0;
  min-width: 0;
  font: 13px/1.6 monospace;
}

.answer-check-btn {
  flex-shrink: 0;
}
.answer-feedback {
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-size: 12px;
  line-height: 1.7;
  min-height: 20px;
  margin-top: 8px;
}
.answer-feedback svg {
  align-self: center;
  flex-shrink: 0;
}
.answer-feedback.success {
  color: var(--success-ink);
}
.answer-feedback.error {
  color: var(--error-ink);
}
.answer-feedback.checking {
  color: var(--muted);
}
.dock-restore {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 35px;
  flex-shrink: 0;
  background: var(--paper);
  color: var(--muted);
  border: 0;
  border-top: 1px solid var(--line);
  cursor: pointer;
  font-size: 11px;
}
.auth-gate-container,
.reading-loading {
  display: grid;
  place-items: center;
  min-height: calc(100dvh - 68px);
  padding: 24px;
  color: var(--muted);
  font-size: 13px;
}
.auth-gate-card {
  width: 100%;
  max-width: 420px;
  padding: 36px;
  border: 1px solid var(--line);
  border-radius: 12px;
  text-align: center;
  background: var(--paper);
}
.auth-gate-card > svg {
  margin: 0 auto 18px;
  color: var(--muted);
}
.auth-gate-card h1 {
  font-size: 21px;
  font-weight: 650;
  margin: 0 0 10px;
  color: var(--ink);
}
.auth-gate-card p {
  font-size: 13px;
  line-height: 1.8;
  margin: 0 0 24px;
}
.gate-btn {
  display: block;
  padding: 10px 14px;
  font-size: 12px;
  border-radius: 6px;
  text-decoration: none;
}
.gate-btn.primary {
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  box-shadow: var(--btn-primary-shadow);
  font-weight: 600;
  margin-bottom: 10px;
}
.gate-btn.secondary {
  border: 1px solid var(--line);
  color: var(--ink);
}
svg {
  flex-shrink: 0;
}
a:focus-visible,
button:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 3px;
}
@media (max-width: 600px) {
  .tool-word {
    display: none;
  }
  .answer-check-btn {
    padding: 10px 12px;
  }
}
</style>
