<script setup lang="ts">
import { NuxtLink } from '#components'
import { computed, ref, watch } from 'vue'
import { useRoute, useFetch, useHead } from '#imports'
import { ArrowLeft, Check, LockKeyhole, Clock3, CheckCircle2 } from 'lucide-vue-next'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import { useAuth } from '~/composables/useAuth'
import type { LearningChapter, ModuleProgress } from '~/shared/learning'
import type { ContentCategory, Difficulty, PrerequisiteModule } from '~/shared/content'

definePageMeta({ layout: 'learning' })
type ChapterItem = LearningChapter
interface ModuleInfo {
  id: string
  slug: string
  title: string
  summary: string
  descriptionMarkdown: string
  difficulty: Difficulty
  category: ContentCategory | null
  learningOutcomes: string[]
}
const route = useRoute()
const { currentUser } = useAuth()
const learnerId = computed(() => currentUser.value?.id || '')
const slug = computed(() => String(route.params.slug || ''))
const {
  data: response,
  pending,
  error
} = await useFetch<{
  ok: boolean
  data: {
    imageUrls?: Record<string, string>
    module: ModuleInfo
    chapters: ChapterItem[]
    prerequisites: Omit<PrerequisiteModule, 'status'>[]
    progress: ModuleProgress | null
  }
}>(() => `/api/modules/${slug.value}`, { key: `learning:module:${slug.value}`, watch: [learnerId] })
const moduleData = computed(() => {
  const data = response.value?.data
  return data
    ? {
        ...data.module,
        chapters: data.chapters,
        prerequisites: data.prerequisites,
        progress: data.progress
      }
    : null
})
const descriptionExpanded = ref(false)
watch(slug, () => {
  descriptionExpanded.value = false
})
function extractPlainText(md: string): string {
  return md
    .replace(/```[\s\S]*?```/g, '')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .replace(/[#*~_>]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

const rawMarkdown = computed(() => moduleData.value?.descriptionMarkdown || '')
const descriptionText = computed(() => extractPlainText(rawMarkdown.value))
const descriptionNeedsExpansion = computed(
  () => descriptionText.value.length > 120 || /```|\|/.test(rawMarkdown.value)
)
const descriptionPreview = computed(() =>
  descriptionText.value.length > 160
    ? `${descriptionText.value.slice(0, 160)}…`
    : descriptionText.value
)
const hasSidebar = computed(
  () =>
    !!moduleData.value &&
    (Boolean(moduleData.value.descriptionMarkdown.trim()) ||
      moduleData.value.learningOutcomes.length > 0 ||
      moduleData.value.prerequisites.length > 0)
)
const firstChapter = computed(
  () =>
    moduleData.value?.chapters.find(
      c => c.slug === moduleData.value?.progress?.resumeChapterSlug
    ) || moduleData.value?.chapters[0]
)
const started = computed(() =>
  moduleData.value?.chapters.some(c => c.lastVisitedAt || c.completedAt)
)
const totalMinutes = computed(() => {
  const list = moduleData.value?.chapters || []
  return list.length && list.every(ch => ch.durationMinutes !== null)
    ? list.reduce((total, ch) => total + ch.durationMinutes!, 0)
    : null
})
const difficultyLabels = { beginner: '基础入门', intermediate: '中级进阶', advanced: '高阶实战' }
function chapterLink(chapter: ChapterItem) {
  const path = `/learn/modules/${slug.value}/${chapter.slug}`
  return chapter.lockReason === 'login' ? `/auth?returnTo=${encodeURIComponent(path)}` : path
}
useHead({
  title: computed(() =>
    moduleData.value ? `${moduleData.value.title} — AttackBox` : '学习模块 — AttackBox'
  ),
  meta: [{ name: 'description', content: computed(() => moduleData.value?.summary || '') }]
})
</script>

<template>
  <div class="module-page">
    <nav class="module-back" aria-label="返回学习模块">
      <NuxtLink to="/learn"><ArrowLeft :size="15" aria-hidden="true" />全部模块</NuxtLink>
    </nav>
    <UiBusyIndicator v-if="pending && !moduleData" class="state-box" label="模块详情加载" />
    <div v-else-if="error || !moduleData" class="state-box">
      <h1>模块暂不可用</h1>
      <NuxtLink to="/learn">返回学习模块</NuxtLink>
    </div>
    <template v-else>
      <header class="hero-section">
        <div class="hero-content">
          <h1>{{ moduleData.title }}</h1>
          <div class="module-meta">
            <span v-if="moduleData.category" class="category-tag">{{
              moduleData.category.name
            }}</span>
            <span>{{ difficultyLabels[moduleData.difficulty] }}</span>
            <span>{{ moduleData.chapters.length }} 章节</span>
            <span v-if="totalMinutes">约 {{ totalMinutes }} 分钟</span>
          </div>
          <p v-if="moduleData.progress && started" class="module-progress-status">
            <CheckCircle2 :size="14" aria-hidden="true" />已完成
            {{ moduleData.progress.completedChapters }} /
            {{ moduleData.progress.totalChapters }} 章节
          </p>
        </div>
        <NuxtLink v-if="firstChapter" :to="chapterLink(firstChapter)" class="start-learning-btn">
          {{
            moduleData.progress?.isCompleted
              ? '回顾模块'
              : started
                ? '继续学习'
                : firstChapter.lockReason === 'login'
                  ? '登录后学习'
                  : '开始学习'
          }}
        </NuxtLink>
      </header>

      <div class="module-content" :class="{ 'with-sidebar': hasSidebar }">
        <section class="chapters-section" aria-labelledby="chapters-heading">
          <header class="chapters-header"><h2 id="chapters-heading">章节</h2></header>
          <div class="chapter-list">
            <component
              :is="chapter.lockReason === 'prerequisite' ? 'button' : NuxtLink"
              :disabled="chapter.lockReason === 'prerequisite' || undefined"
              :type="chapter.lockReason === 'prerequisite' ? 'button' : undefined"
              v-for="(chapter, index) in moduleData.chapters"
              :key="chapter.id"
              :to="chapter.lockReason === 'prerequisite' ? undefined : chapterLink(chapter)"
              class="chapter-row"
              :aria-disabled="chapter.lockReason === 'prerequisite' || undefined"
              @click="chapter.lockReason === 'prerequisite' && $event.preventDefault()"
            >
              <span class="chapter-number">{{ String(index + 1).padStart(2, '0') }}</span>
              <div class="chapter-info">
                <h3>{{ chapter.title }}</h3>
                <div
                  v-if="
                    chapter.questionsCount ||
                    chapter.isLocked ||
                    chapter.durationMinutes ||
                    chapter.completedAt
                  "
                  class="chapter-meta"
                >
                  <span v-if="chapter.durationMinutes" class="access-label"
                    ><Clock3 :size="13" aria-hidden="true" />{{
                      chapter.durationMinutes
                    }}
                    分钟</span
                  >
                  <span v-if="chapter.questionsCount">{{ chapter.questionsCount }} 道练习</span>
                  <span v-if="chapter.isLocked" class="access-label"
                    ><LockKeyhole :size="13" aria-hidden="true" />{{
                      chapter.lockReason === 'login' ? '需登录' : '需先完成前章'
                    }}</span
                  ><span v-if="chapter.completedAt" class="access-label chapter-completed"
                    ><CheckCircle2 :size="13" aria-hidden="true" />已完成</span
                  >
                </div>
              </div>
            </component>
          </div>
        </section>

        <aside v-if="hasSidebar" class="module-sidebar" aria-label="模块信息">
          <section
            v-if="moduleData.descriptionMarkdown.trim()"
            class="support-section module-description"
            aria-labelledby="details-heading"
          >
            <h2 id="details-heading">详情</h2>
            <div id="module-description-content">
              <p
                v-if="descriptionNeedsExpansion && !descriptionExpanded && descriptionPreview"
                class="description-excerpt"
              >
                {{ descriptionPreview }}
              </p>
              <MarkdownView
                v-else-if="!descriptionNeedsExpansion || descriptionExpanded"
                :content="moduleData.descriptionMarkdown"
                :image-urls="response?.data.imageUrls"
              />
            </div>
            <button
              v-if="descriptionNeedsExpansion"
              type="button"
              class="description-toggle"
              :aria-expanded="descriptionExpanded"
              aria-controls="module-description-content"
              @click="descriptionExpanded = !descriptionExpanded"
            >
              {{ descriptionExpanded ? '收起详情' : '展开详情' }}
            </button>
          </section>
          <section
            v-if="moduleData.learningOutcomes.length"
            class="support-section outcomes"
            aria-labelledby="outcomes-heading"
          >
            <h2 id="outcomes-heading">目标</h2>
            <ul>
              <li v-for="(outcome, index) in moduleData.learningOutcomes" :key="index">
                <Check :size="15" aria-hidden="true" /><span>{{ outcome }}</span>
              </li>
            </ul>
          </section>
          <section
            v-if="moduleData.prerequisites.length"
            class="support-section prerequisites"
            aria-labelledby="prerequisites-heading"
          >
            <h2 id="prerequisites-heading">前置模块</h2>
            <ol class="prerequisite-chain">
              <li v-for="(prerequisite, index) in moduleData.prerequisites" :key="prerequisite.id">
                <NuxtLink :to="`/learn/modules/${prerequisite.slug}`"
                  ><span class="chain-number">{{ index + 1 }}</span
                  ><span>{{ prerequisite.title }}</span></NuxtLink
                >
              </li>
            </ol>
          </section>
        </aside>
      </div>
    </template>
  </div>
</template>

<style scoped>
.module-progress-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
  margin: 14px 0 0;
}
.chapter-completed {
  color: var(--success-ink);
}
.chapter-row[aria-disabled='true'] {
  opacity: 0.6;
  cursor: not-allowed;
}
.module-page {
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  padding: 28px 32px 80px;
  color: var(--ink);
}
.module-back {
  margin-bottom: 36px;
}
.module-back a {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 13px;
  text-decoration: none;
}
.module-back a:hover {
  color: var(--ink);
}
.hero-section {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 36px;
  padding: 28px 32px;
  background: linear-gradient(135deg, var(--paper) 0%, var(--soft) 100%);
  border: 1px solid var(--line);
  border-radius: 14px;
  box-shadow: var(--surface-highlight), var(--card-shadow);
  animation: surface-rise 0.3s cubic-bezier(0.22, 1, 0.36, 1) both;
}
.hero-content {
  min-width: 0;
}
h1 {
  font-size: clamp(28px, 3.3vw, 38px);
  font-weight: 700;
  line-height: 1.3;
  letter-spacing: -0.7px;
  margin: 0;
  overflow-wrap: anywhere;
}
.module-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 0;
  color: var(--muted);
  font-size: 12px;
  margin-top: 18px;
  line-height: 1.5;
}
.module-meta > span {
  display: inline-flex;
  align-items: center;
}
.module-meta > span + span::before {
  content: '';
  width: 3px;
  height: 3px;
  margin: 0 13px;
  border-radius: 50%;
  background: var(--muted);
  opacity: 0.45;
}
.module-meta .category-tag {
  color: var(--ink);
}
.start-learning-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  min-height: 44px;
  padding: 12px 26px;
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  box-shadow: var(--btn-primary-shadow);
  border: 1px solid transparent;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  transition:
    transform 0.16s ease,
    background 0.16s ease;
}
.start-learning-btn:hover {
  transform: translateY(-1px);
  background: var(--btn-primary-hover);
}
.module-content {
  padding-top: 32px;
}
.module-content.with-sidebar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 40px;
  align-items: start;
}
.chapters-section {
  min-width: 0;
}
.chapters-header {
  padding: 0 0 16px;
  border-bottom: 1px solid var(--line);
}
.chapters-header h2 {
  font-size: 19px;
  font-weight: 650;
  margin: 0;
  letter-spacing: -0.2px;
}
.chapter-list {
  padding: 0;
}
.chapter-row {
  width: 100%;
  background: transparent;
  border: 0;
  text-align: left;
  cursor: pointer;
  display: flex;
  align-items: flex-start;
  gap: 18px;
  padding: 16px 0;
  border-bottom: 1px solid var(--line);
  text-decoration: none;
  color: inherit;
  position: relative;
}
.chapter-row:last-child {
  border-bottom: 0;
}
.chapter-row::before {
  content: '';
  position: absolute;
  inset: 3px -10px;
  border-radius: 7px;
  background: var(--soft);
  opacity: 0;
  transition: opacity 0.15s;
}
.chapter-row:hover::before {
  opacity: 1;
}
.chapter-row > * {
  position: relative;
}
.chapter-number {
  font-family: monospace;
  color: var(--muted);
  font-size: 12px;
  flex-shrink: 0;
  line-height: 24px;
}
.chapter-info {
  flex: 1;
  min-width: 0;
}
h3 {
  font-size: 15px;
  line-height: 1.5;
  margin: 0;
  font-weight: 550;
  overflow-wrap: anywhere;
}
.chapter-row:hover h3 {
  color: var(--green);
}
.chapter-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
  color: var(--muted);
  font-size: 12px;
  margin-top: 6px;
  line-height: 1.6;
}
.access-label {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.module-sidebar {
  min-width: 0;
  padding: 4px 0;
}
.support-section + .support-section {
  margin-top: 24px;
}
.support-section h2 {
  font-size: 14px;
  font-weight: 650;
  margin: 0 0 14px;
}
.module-description h2 {
  color: var(--muted);
  font-size: 13px;
}
.description-excerpt {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.85;
  margin: 0;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 4;
  overflow: hidden;
  overflow-wrap: anywhere;
}
.description-toggle {
  display: inline-block;
  margin-top: 10px;
  padding: 2px 0;
  border: 0;
  border-bottom: 1px solid var(--line);
  background: transparent;
  color: var(--ink);
  font-size: 12px;
  cursor: pointer;
}
.description-toggle:hover {
  border-bottom-color: var(--ink);
}
.description-toggle:focus-visible {
  outline: 2px solid var(--green);
  outline-offset: 4px;
}
.module-description :deep(.markdown-body) {
  font-size: 13px;
  line-height: 1.85;
  color: var(--muted);
}
.module-description :deep(.markdown-body > :first-child) {
  margin-top: 0;
}
.module-description :deep(.markdown-body > :last-child) {
  margin-bottom: 0;
}
.module-description :deep(.markdown-body p) {
  color: var(--muted);
  opacity: 1;
}
.module-description :deep(.markdown-body h1),
.module-description :deep(.markdown-body h2),
.module-description :deep(.markdown-body h3) {
  font-size: 14px;
  margin: 18px 0 8px;
  padding: 0;
  border: 0;
}
.module-description :deep(.markdown-body pre) {
  font-size: 12px;
  padding: 12px;
}
.module-description :deep(.markdown-body .code-block) {
  margin: 14px 0;
}
.module-description :deep(.markdown-body .code-top) {
  padding: 7px 10px;
}
.module-description :deep(.markdown-body table) {
  display: block;
  overflow-x: auto;
  max-width: 100%;
}
.outcomes {
  padding: 18px 20px;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 9px;
}
.outcomes h2 {
  font-size: 15px;
  margin-bottom: 12px;
}
.outcomes ul {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 10px;
}
.outcomes li {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  font-size: 13px;
  line-height: 1.75;
  color: var(--ink);
  overflow-wrap: anywhere;
}
.outcomes li svg {
  flex-shrink: 0;
  margin-top: 4px;
  color: var(--green);
}
.prerequisites {
  padding-top: 20px;
  border-top: 1px solid var(--line);
}
.prerequisites:first-child {
  padding-top: 0;
  border-top: 0;
}
.prerequisites h2 {
  font-size: 13px;
  color: var(--muted);
  margin-bottom: 10px;
}
.prerequisite-chain {
  list-style: none;
  padding: 0;
  margin: 0;
}
.prerequisite-chain li {
  position: relative;
  padding-bottom: 8px;
}
.prerequisite-chain li:last-child {
  padding-bottom: 0;
}
.prerequisite-chain a {
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
}
.prerequisite-chain a:hover {
  color: var(--green);
}
.prerequisite-chain a:hover > span:last-child {
  text-decoration: underline;
  text-underline-offset: 3px;
}
.chain-number {
  display: grid;
  place-items: center;
  width: 14px;
  height: 20px;
  flex-shrink: 0;
  color: var(--muted);
  font-size: 11px;
  font-family: monospace;
}
.prerequisite-chain a > span:last-child {
  overflow-wrap: anywhere;
}
.state-box {
  padding: 60px 0;
  color: var(--muted);
}
.state-box a {
  color: var(--green);
}
a:focus-visible {
  outline: 2px solid var(--green);
  outline-offset: 4px;
  border-radius: 5px;
}
.chapter-row:focus-visible {
  outline-offset: -3px;
}
@media (max-width: 800px) {
  .module-page {
    padding: 24px 20px 56px;
  }
  .module-back {
    margin-bottom: 28px;
  }
  .hero-section {
    flex-direction: column;
    align-items: flex-start;
    gap: 22px;
    padding-bottom: 26px;
  }
  .module-meta {
    margin-top: 14px;
  }
  .module-content.with-sidebar {
    grid-template-columns: 1fr;
    gap: 30px;
  }
  .chapters-header {
    padding: 0 0 16px;
  }
  .chapter-row {
    gap: 15px;
    padding: 16px 0;
  }
  .module-sidebar {
    padding: 0 4px;
  }
}
</style>
