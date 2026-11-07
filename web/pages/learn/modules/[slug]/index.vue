<script setup lang="ts">
import { NuxtLink } from '#components'
import { computed, ref, watch } from 'vue'
import {
  useRoute,
  useFetch,
  useHead,
  useRequestEvent,
  setResponseStatus
} from '#imports'
import { ArrowLeft, Check, LockKeyhole, Clock3, CheckCircle2 } from 'lucide-vue-next'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import { useAuth } from '~/composables/useAuth'
import type { LearningChapter, ModuleProgress } from '~/shared/learning'
import type { ContentCategory, Difficulty, PrerequisiteModule } from '~/shared/content'
import {
  buildKeywords,
  canonicalUrl,
  difficultySeoLabel,
  extractSeoExcerpt,
  serializeJsonLd
} from '~/shared/seo'
import { useSiteOrigin } from '~/composables/useSiteOrigin'

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
if (import.meta.server && error.value) {
  const event = useRequestEvent()
  if (event) setResponseStatus(event, error.value.statusCode || 404)
}
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
    .replace(/!\[([^\]]*)\]\([^)]+\)/g, '$1')
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
const isPrivateModule = computed(
  () => !!moduleData.value?.chapters.length && moduleData.value.chapters.every(c => c.access === 'authenticated')
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
const siteOrigin = useSiteOrigin()
const moduleCanonical = computed(() =>
  canonicalUrl(siteOrigin, `/learn/modules/${encodeURIComponent(slug.value)}`)
)
const moduleSeoDescription = computed(
  () =>
    moduleData.value?.summary ||
    extractSeoExcerpt(rawMarkdown.value, 150) ||
    '在 AttackBox 学习网络安全模块，通过分步章节与实验环境验证安全原理。'
)
const moduleKeywords = computed(() =>
  buildKeywords(
    moduleData.value?.title,
    moduleData.value?.category?.name,
    difficultySeoLabel(moduleData.value?.difficulty),
    moduleData.value?.chapters.slice(0, 6).map(ch => ch.title),
    '网络安全课程',
    'AttackBox'
  )
)
const moduleJsonLd = computed(() => {
  if (!moduleData.value) return ''
  return serializeJsonLd({
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'Course',
        name: moduleData.value.title,
        description: moduleSeoDescription.value,
        url: moduleCanonical.value,
        inLanguage: 'zh-CN',
        isAccessibleForFree: !isPrivateModule.value,
        educationalLevel: difficultySeoLabel(moduleData.value.difficulty),
        provider: {
          '@type': 'Organization',
          name: 'AttackBox',
          url: siteOrigin
        },
        ...(moduleData.value.learningOutcomes.length
          ? { teaches: moduleData.value.learningOutcomes }
          : {}),
        hasPart: moduleData.value.chapters.map((ch, idx) => ({
          '@type': 'LearningResource',
          position: idx + 1,
          name: ch.title,
          ...(ch.access === 'public'
            ? {
                url: canonicalUrl(
                  siteOrigin,
                  `/learn/modules/${encodeURIComponent(slug.value)}/${encodeURIComponent(ch.slug)}`
                )
              }
            : {})
        }))
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
            item: moduleCanonical.value
          }
        ]
      }
    ]
  })
})
useHead(() => ({
  title: moduleData.value ? `${moduleData.value.title} — AttackBox` : '学习模块 — AttackBox',
  meta: [
    { name: 'description', content: moduleSeoDescription.value },
    { name: 'keywords', content: moduleKeywords.value },
    {
      property: 'og:title',
      content: moduleData.value
        ? `${moduleData.value.title} — AttackBox`
        : '学习模块 — AttackBox'
    },
    { property: 'og:description', content: moduleSeoDescription.value },
    { property: 'og:type', content: 'article' },
    { property: 'og:url', content: moduleCanonical.value },
    ...(error.value ? [{ name: 'robots', content: 'noindex, nofollow' }] : [])
  ],
  link: [{ rel: 'canonical', href: moduleCanonical.value }],
  script: moduleJsonLd.value
    ? [{ type: 'application/ld+json', innerHTML: moduleJsonLd.value }]
    : []
}))
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
            <span>{{ isPrivateModule ? '登录模块' : '公开模块' }}</span>
            <span>{{ moduleData.chapters.length }} 章节</span>
            <span v-if="totalMinutes">约 {{ totalMinutes }} 分钟</span>
          </div>
          <p v-if="moduleData.progress && started" class="module-progress-status">
            <CheckCircle2 :size="14" aria-hidden="true" />已完成
            {{ moduleData.progress.completedChapters }} /
            {{ moduleData.progress.totalChapters }} 章节
          </p>
        </div>
        <NuxtLink
          v-if="firstChapter"
          :to="chapterLink(firstChapter)"
          :rel="firstChapter.lockReason === 'login' ? 'nofollow' : undefined"
          class="start-learning-btn"
        >
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
            <NuxtLink
              v-for="(chapter, index) in moduleData.chapters"
              :key="chapter.id"
              :to="chapterLink(chapter)"
              :rel="chapter.lockReason === 'login' ? 'nofollow' : undefined"
              class="chapter-row"
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
                    ><LockKeyhole :size="13" aria-hidden="true" />需登录</span
                  ><span v-if="chapter.completedAt" class="access-label chapter-completed"
                    ><CheckCircle2 :size="13" aria-hidden="true" />已完成</span
                  >
                </div>
              </div>
            </NuxtLink>
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
              <div
                v-show="!descriptionNeedsExpansion || descriptionExpanded"
                class="description-full"
              >
                <MarkdownView
                  :content="moduleData.descriptionMarkdown"
                  :image-urls="response?.data.imageUrls"
                />
              </div>
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
  color: var(--brand);
  font-weight: 550;
  margin: 14px 0 0;
}
.chapter-completed {
  color: var(--success-ink);
}
.module-page {
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  padding: 28px 32px 80px;
  color: var(--ink);
}
.module-back {
  margin-bottom: 28px;
}
.module-back a {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 7px;
  border: 1px solid var(--line);
  background: var(--card-surface);
  box-shadow: var(--surface-highlight);
  color: var(--muted);
  font-size: 12.5px;
  font-weight: 550;
  text-decoration: none;
  transition:
    color 0.16s ease,
    border-color 0.16s ease;
}
.module-back a:hover {
  color: var(--ink);
  border-color: var(--line-strong);
}
.hero-section {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 36px;
  padding: 32px 36px;
  background:
    radial-gradient(ellipse 55% 110% at 92% 10%, var(--brand-soft), transparent 72%),
    var(--card-surface);
  border: 1px solid var(--line-strong, var(--line));
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
  line-height: 1.28;
  letter-spacing: -0.8px;
  margin: 0;
  overflow-wrap: anywhere;
}
.module-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 0;
  color: var(--muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  margin-top: 16px;
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
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  color: var(--brand);
  font-weight: 600;
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
  font-weight: 650;
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
  gap: 32px;
  align-items: start;
}
.chapters-section {
  min-width: 0;
}
.chapters-header {
  padding: 0 4px 14px;
}
.chapters-header h2 {
  font-size: 18px;
  font-weight: 650;
  margin: 0;
  letter-spacing: -0.3px;
}
.chapter-list {
  display: grid;
  gap: 8px;
  padding: 8px;
  border-radius: 13px;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  box-shadow: var(--well-shadow);
}
.chapter-row {
  width: 100%;
  box-sizing: border-box;
  background: var(--card-surface);
  border: 1px solid var(--line);
  border-radius: 9px;
  box-shadow: var(--surface-highlight);
  text-align: left;
  cursor: pointer;
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 16px 18px;
  text-decoration: none;
  color: inherit;
  position: relative;
  transition:
    border-color 0.18s ease,
    transform 0.18s ease,
    box-shadow 0.18s ease;
}
.chapter-row:hover {
  border-color: var(--brand-border);
  transform: translateY(-1px);
  box-shadow: var(--surface-highlight), 0 8px 20px -6px rgba(0, 0, 0, 0.5);
}
.chapter-number {
  display: inline-grid;
  place-items: center;
  min-width: 28px;
  height: 24px;
  padding: 0 6px;
  border-radius: 5px;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  color: var(--muted);
  font-size: 11px;
  font-weight: 600;
  flex-shrink: 0;
}
.chapter-row:hover .chapter-number {
  color: var(--brand);
  border-color: var(--brand-border);
}
.chapter-info {
  flex: 1;
  min-width: 0;
}
h3 {
  font-size: 15px;
  line-height: 1.5;
  margin: 0;
  font-weight: 600;
  overflow-wrap: anywhere;
  transition: color 0.16s ease;
}
.chapter-row:hover h3 {
  color: var(--brand);
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
  display: grid;
  gap: 18px;
}
.support-section {
  padding: 20px 22px;
  background: var(--card-surface);
  border: 1px solid var(--line);
  border-radius: 12px;
  box-shadow: var(--surface-highlight), var(--card-shadow);
}
.support-section h2 {
  font-size: 14px;
  font-weight: 650;
  margin: 0 0 12px;
  letter-spacing: -0.2px;
}
.module-description h2 {
  color: var(--ink);
}
.module-description {
  --markdown-font-size: 13px;
  --markdown-ink: var(--muted);
}
.description-excerpt {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.85;
  margin: 0;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 4;
  line-clamp: 4;
  overflow: hidden;
  overflow-wrap: anywhere;
}
.description-toggle {
  display: inline-block;
  margin-top: 12px;
  padding: 2px 0;
  border: 0;
  border-bottom: 1px solid var(--brand-border);
  background: transparent;
  color: var(--brand);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}
.description-toggle:hover {
  border-bottom-color: var(--brand);
}
.description-toggle:focus-visible {
  outline: 2px solid var(--brand);
  outline-offset: 4px;
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
  line-height: 1.7;
  color: var(--body);
  overflow-wrap: anywhere;
}
.outcomes li svg {
  flex-shrink: 0;
  margin-top: 4px;
  color: var(--brand);
}
.prerequisite-chain {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 8px;
}
.prerequisite-chain a {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 7px;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  text-decoration: none;
  color: var(--body);
  font-size: 12.5px;
  line-height: 1.5;
  transition:
    color 0.16s ease,
    border-color 0.16s ease;
}
.prerequisite-chain a:hover {
  color: var(--brand);
  border-color: var(--brand-border);
}
.chain-number {
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  border-radius: 4px;
  background: var(--soft);
  flex-shrink: 0;
  color: var(--muted);
  font-size: 10.5px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.prerequisite-chain a > span:last-child {
  overflow-wrap: anywhere;
}
.state-box {
  padding: 60px 24px;
  text-align: center;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  box-shadow: var(--well-shadow);
  border-radius: 13px;
  color: var(--muted);
}
.state-box a {
  color: var(--brand);
}
a:focus-visible {
  outline: 2px solid var(--brand);
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
    margin-bottom: 24px;
  }
  .hero-section {
    flex-direction: column;
    align-items: flex-start;
    gap: 22px;
    padding: 24px;
  }
  .module-meta {
    margin-top: 14px;
  }
  .module-content.with-sidebar {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .chapter-row {
    gap: 14px;
    padding: 14px;
  }
}
</style>
