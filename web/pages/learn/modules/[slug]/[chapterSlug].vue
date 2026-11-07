<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import {
  useRoute,
  useFetch,
  useHead,
  useRequestEvent,
  setResponseStatus,
  navigateTo
} from '#imports'
import {
  ArrowRight,
  LockKeyhole,
  Clock3,
  CheckCircle2,
  CircleAlert,
  BookOpen
} from 'lucide-vue-next'
import ReaderLayout from '~/components/reader/Layout.vue'
import ReaderArticle from '~/components/reader/Article.vue'
import ChapterOutline from '~/components/learning/ChapterOutline.vue'
import PracticeDock from '~/components/learning/PracticeDock.vue'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import MarkdownContent from '~/components/ui/MarkdownContent.vue'
import { useAuth } from '~/composables/useAuth'
import type { LearningTarget } from '~/shared/learning-target'
import type { ChapterProgress, LearningChapter } from '~/shared/learning'
import {
  buildKeywords,
  canonicalUrl,
  extractSeoExcerpt,
  serializeJsonLd
} from '~/shared/seo'
import { useSiteOrigin } from '~/composables/useSiteOrigin'
import { renderMarkdown, type MarkdownHeading } from '~/utils/markdown'
import { apiErrorMessage, apiFetch } from '~/utils/api'

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
  questions: (QuestionItem & { promptHtml: string })[]
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
const loginUrl = computed(() => '/auth?returnTo=' + encodeURIComponent(route.fullPath))

function redirectToLogin() {
  return navigateTo(loginUrl.value)
}

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
if (import.meta.server && chapterError.value) {
  const event = useRequestEvent()
  if (event) setResponseStatus(event, chapterError.value.statusCode || 404)
}

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
useReadingAnalytics(scrollElement)
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
const hasPractice = computed(
  () => !!chapterData.value?.questions.length || !!chapterResponse.value?.data.target
)
const workspace = computed(() => readerLayout.value?.workspaceElement ?? null)
const dock = usePracticeLayout(workspace, hasPractice)

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
  if (!learnerId.value) {
    await redirectToLogin()
    return
  }
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
  if (!learnerId.value) {
    await redirectToLogin()
    return
  }
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

const siteOrigin = useSiteOrigin()
const chapterCanonical = computed(() =>
  canonicalUrl(
    siteOrigin,
    `/learn/modules/${encodeURIComponent(moduleSlug.value)}/${encodeURIComponent(
      chapterSlug.value
    )}`
  )
)
const chapterDescription = computed(
  () =>
    extractSeoExcerpt(chapterData.value?.bodyMarkdown, 155) ||
    moduleData.value?.summary ||
    '在 AttackBox 阅读网络安全实训章节并完成动手验证。'
)
const chapterRobots = computed(() => {
  const access = chapterResponse.value?.data?.chapter.access
  if (chapterError.value || access === 'authenticated') {
    return 'noindex, nofollow'
  }
  return 'index, follow'
})
const chapterKeywords = computed(() =>
  buildKeywords(
    chapterData.value?.title,
    moduleData.value?.title,
    sections.value.slice(0, 6).map(s => s.text),
    '网络安全教程',
    'AttackBox'
  )
)
const chapterJsonLd = computed(() => {
  if (!chapterData.value || !moduleData.value || chapterRobots.value.startsWith('noindex')) {
    return ''
  }
  const moduleUrl = canonicalUrl(
    siteOrigin,
    `/learn/modules/${encodeURIComponent(moduleSlug.value)}`
  )
  return serializeJsonLd({
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'TechArticle',
        headline: chapterData.value.title,
        description: chapterDescription.value,
        url: chapterCanonical.value,
        inLanguage: 'zh-CN',
        isPartOf: {
          '@type': 'Course',
          name: moduleData.value.title,
          url: moduleUrl
        },
        author: {
          '@type': 'Organization',
          name: 'AttackBox',
          url: siteOrigin
        }
      },
      {
        '@type': 'BreadcrumbList',
        itemListElement: [
          { '@type': 'ListItem', position: 1, name: '首页', item: `${siteOrigin}/` },
          { '@type': 'ListItem', position: 2, name: '学习', item: `${siteOrigin}/learn` },
          {
            '@type': 'ListItem',
            position: 3,
            name: moduleData.value.title,
            item: moduleUrl
          },
          {
            '@type': 'ListItem',
            position: 4,
            name: chapterData.value.title,
            item: chapterCanonical.value
          }
        ]
      }
    ]
  })
})

useHead(() => ({
  meta: [
    { name: 'description', content: chapterDescription.value },
    { name: 'keywords', content: chapterKeywords.value },
    { name: 'robots', content: chapterRobots.value },
    {
      property: 'og:title',
      content:
        chapterData.value && moduleData.value
          ? `${chapterData.value.title} — ${moduleData.value.title} — AttackBox`
          : '学习章节 — AttackBox'
    },
    { property: 'og:description', content: chapterDescription.value },
    { property: 'og:type', content: 'article' },
    { property: 'og:url', content: chapterCanonical.value }
  ],
  link: [{ rel: 'canonical', href: chapterCanonical.value }],
  script: chapterJsonLd.value
    ? [{ type: 'application/ld+json', innerHTML: chapterJsonLd.value }]
    : []
}))

useHead({
  title: computed(() => {
    if (chapterData.value && moduleData.value) {
      return `${chapterData.value.title} — ${moduleData.value.title} — AttackBox`
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
          {{ chapterError?.statusCode === 401 ? '登录后继续学习' : '章节暂不可用' }}
        </h1>
        <p>
          {{
            chapterError?.statusCode === 401
              ? '当前模块需要登录账号后阅读。'
              : '该章节可能已更新或下架。'
          }}
        </p>
        <NuxtLink
          v-if="chapterError?.statusCode === 401"
          :to="loginUrl"
          rel="nofollow"
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
      :outline-to="`/learn/modules/${moduleSlug}`"
      :side-practice="dock.sidePractice.value"
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
        <LearningPracticeEnvironment
          v-if="hasPractice"
          :key="chapterResponse?.data.chapter.id"
          :target="chapterResponse?.data.target"
          :context="{
            kind: 'chapter',
            moduleSlug: String(route.params.slug),
            chapterSlug: String(route.params.chapterSlug)
          }"
          @open="dock.open"
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
              <MarkdownContent class="q-prompt" :html="q.promptHtml" />
            </div>
            <div v-if="!savedProgress.completedAt" class="answer-line">
              <UiInput
                :id="`answer-${q.id}`"
                v-model="userAnswers[q.id]"
                bare
                type="text"
                class="answer-input"
                :placeholder="learnerId ? '输入你的答案' : '登录后输入答案'"
                maxlength="2000"
                autocomplete="off"
                autocapitalize="off"
                spellcheck="false"
                :aria-labelledby="`question-${q.id}`"
                :aria-describedby="`feedback-${q.id}`"
                :aria-invalid="answerFeedbacks[q.id]?.status === 'error'"
                :disabled="answerFeedbacks[q.id]?.status === 'checking'"
                @focus="!learnerId && redirectToLogin()"
                @click="!learnerId && redirectToLogin()"
                @input="delete answerFeedbacks[q.id]"
              />
              <UiButton
                type="submit"
                variant="secondary"
                class="answer-check-btn"
                :loading="answerFeedbacks[q.id]?.status === 'checking'"
                >{{ learnerId ? '校验答案' : '登录后答题' }}</UiButton
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
              :to="loginUrl"
              rel="nofollow"
              class="chapter-primary"
              >登录后保存进度</NuxtLink
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
        <PracticeDock v-if="dock.opened.value" :key="learnerId" :layout="dock" />
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
  font-size: var(--reader-title-size);
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
.auth-gate-container,
.reading-loading {
  display: grid;
  place-items: center;
  min-height: calc(100dvh - var(--site-header-height));
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
  .answer-check-btn {
    padding: 10px 12px;
  }
}
</style>
