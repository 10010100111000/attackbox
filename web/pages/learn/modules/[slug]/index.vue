<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useFetch, useHead } from '#imports'
import { ArrowLeft, ArrowRight, BookOpen, Check, ChevronRight, LockKeyhole, Clock3 } from 'lucide-vue-next'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import type { ContentCategory, Difficulty, PrerequisiteModule } from '~/shared/content'

definePageMeta({ layout: 'learning' })
interface ChapterItem {
  id: string; slug: string; title: string; access: 'public' | 'authenticated'
  position: number; durationMinutes: number | null; questionsCount: number; isLocked: boolean
}
interface ModuleInfo {
  id: string; slug: string; title: string; summary: string; descriptionMarkdown: string
  difficulty: Difficulty; category: ContentCategory | null; learningOutcomes: string[]
}
const route = useRoute()
const slug = computed(() => String(route.params.slug || ''))
const { data: response, pending, error } = await useFetch<{
  ok: boolean; data: { module: ModuleInfo; chapters: ChapterItem[]; prerequisites: Omit<PrerequisiteModule, 'status'>[] }
}>(() => `/api/modules/${slug.value}`)
const moduleData = computed(() => {
  const data = response.value?.data
  return data ? { ...data.module, chapters: data.chapters, prerequisites: data.prerequisites } : null
})
const firstChapter = computed(() => moduleData.value?.chapters[0])
const totalMinutes = computed(() => {
  const list = moduleData.value?.chapters || []
  return list.length && list.every(ch => ch.durationMinutes !== null) ? list.reduce((total, ch) => total + ch.durationMinutes!, 0) : null
})
const difficultyLabels = { beginner: '基础入门', intermediate: '中级进阶', advanced: '高阶实战' }
function chapterLink(chapter: ChapterItem) {
  const path = `/learn/modules/${slug.value}/${chapter.slug}`
  return chapter.isLocked ? `/auth?returnTo=${encodeURIComponent(path)}` : path
}
useHead({
  title: computed(() => moduleData.value ? `${moduleData.value.title} — AttackBox` : '学习模块 — AttackBox'),
  meta: [{ name: 'description', content: computed(() => moduleData.value?.summary || '') }],
})
</script>

<template>
  <div class="module-page">
    <nav class="crumb" aria-label="面包屑">
      <NuxtLink to="/learn"><ArrowLeft :size="15" aria-hidden="true" />学习模块</NuxtLink>
      <ChevronRight :size="13" aria-hidden="true" />
      <span>{{ moduleData?.title || slug }}</span>
    </nav>
    <div v-if="pending" class="state-box" role="status">正在加载模块详情…</div>
    <div v-else-if="error || !moduleData" class="state-box">
      <h1>模块暂不可用</h1>
      <NuxtLink to="/learn">返回学习模块</NuxtLink>
    </div>
    <template v-else>
      <header class="hero-section">
        <div class="hero-content">
          <div class="module-meta">
            <span v-if="moduleData.category" class="category-tag">{{ moduleData.category.name }}</span>
            <span>{{ difficultyLabels[moduleData.difficulty] }}</span>
            <span class="chapter-count"><BookOpen :size="15" aria-hidden="true" />{{ moduleData.chapters.length }} 章节</span>
            <span v-if="totalMinutes" class="chapter-count"><Clock3 :size="15" aria-hidden="true" />约 {{ totalMinutes }} 分钟</span>
          </div>
          <h1>{{ moduleData.title }}</h1>
          <p v-if="moduleData.summary" class="hero-desc">{{ moduleData.summary }}</p>
        </div>
        <NuxtLink v-if="firstChapter" :to="chapterLink(firstChapter)" class="start-learning-btn">
          {{ firstChapter.isLocked ? '登录后学习' : '开始学习' }}<ArrowRight :size="17" aria-hidden="true" />
        </NuxtLink>
      </header>

      <div class="module-content" :class="{ 'with-outcomes': moduleData.learningOutcomes.length || moduleData.prerequisites.length }">
        <div class="main-content">
          <section v-if="moduleData.descriptionMarkdown.trim()" class="module-description" aria-label="模块介绍">
            <MarkdownView :content="moduleData.descriptionMarkdown" />
          </section>
          <section class="chapters-section" aria-labelledby="chapters-heading">
            <h2 id="chapters-heading">章节内容</h2>
            <div class="chapter-list">
              <NuxtLink v-for="(chapter, index) in moduleData.chapters" :key="chapter.id"
                :to="chapterLink(chapter)" class="chapter-row">
                <span class="chapter-number">{{ String(index + 1).padStart(2, '0') }}</span>
                <div class="chapter-info">
                  <h3>{{ chapter.title }}</h3>
                  <div v-if="chapter.questionsCount || chapter.isLocked || chapter.durationMinutes" class="chapter-meta">
                    <span v-if="chapter.durationMinutes" class="access-label"><Clock3 :size="13" aria-hidden="true" />{{ chapter.durationMinutes }} 分钟</span>
                    <span v-if="chapter.questionsCount">{{ chapter.questionsCount }} 道练习</span>
                    <span v-if="chapter.isLocked" class="access-label"><LockKeyhole :size="13" aria-hidden="true" />需登录</span>
                  </div>
                </div>
                <ChevronRight :size="18" class="chapter-arrow" aria-hidden="true" />
              </NuxtLink>
            </div>
          </section>
        </div>
        <aside v-if="moduleData.learningOutcomes.length || moduleData.prerequisites.length" class="module-sidebar">
        <section v-if="moduleData.learningOutcomes.length" class="outcomes" aria-labelledby="outcomes-heading">
          <h2 id="outcomes-heading">学完后，你将能够</h2>
          <ul>
            <li v-for="(outcome, index) in moduleData.learningOutcomes" :key="index">
              <Check :size="16" aria-hidden="true" /><span>{{ outcome }}</span>
            </li>
          </ul>
        </section>
        <section v-if="moduleData.prerequisites.length" class="prerequisites" aria-labelledby="prerequisites-heading">
          <h2 id="prerequisites-heading">前置学习模块</h2>
          <ol class="prerequisite-chain">
            <li v-for="(prerequisite, index) in moduleData.prerequisites" :key="prerequisite.id">
              <NuxtLink :to="`/learn/modules/${prerequisite.slug}`"><span class="chain-number">{{ index + 1 }}</span><span>{{ prerequisite.title }}</span><ChevronRight :size="14" aria-hidden="true" /></NuxtLink>
            </li>
          </ol>
        </section>
        </aside>
      </div>
    </template>
  </div>
</template>

<style scoped>
.module-page { width: 100%; max-width: 1150px; margin: 0 auto; padding: 32px 32px 80px; color: var(--ink); }
.crumb { display: flex; align-items: center; gap: 10px; color: var(--muted); font-size: 13px; margin-bottom: 48px; }
.crumb a { display: inline-flex; align-items: center; gap: 8px; color: inherit; text-decoration: none; white-space: nowrap; }
.crumb > span { overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.crumb a:hover { color: var(--green); }
.hero-section { display: flex; align-items: center; justify-content: space-between; gap: 40px; padding-bottom: 36px; border-bottom: 1px solid var(--line); }
.hero-content { max-width: 740px; min-width: 0; }
.module-meta { display: flex; gap: 18px; align-items: center; flex-wrap: wrap; color: var(--muted); font-size: 13px; margin-bottom: 18px; }
.category-tag { color: var(--green); }
.chapter-count { display: inline-flex; align-items: center; gap: 7px; }
h1 { font-size: clamp(28px, 4vw, 40px); line-height: 1.25; letter-spacing: -.7px; margin: 0; overflow-wrap: anywhere; }
.hero-desc { color: var(--muted); font-size: 15px; line-height: 1.8; margin: 16px 0 0; white-space: pre-line; }
.start-learning-btn { display: inline-flex; align-items: center; justify-content: center; gap: 16px; flex-shrink: 0; padding: 13px 22px; background: var(--green); color: var(--on-accent); border-radius: 8px; font-size: 14px; font-weight: 600; text-decoration: none; transition: opacity .15s; }
.start-learning-btn:hover { opacity: .85; }
.module-content { padding-top: 36px; }
.module-content.with-outcomes { display: grid; grid-template-columns: minmax(0, 1fr) 280px; gap: 56px; align-items: start; }
.main-content { min-width: 0; }
.module-description { margin-bottom: 36px; }
h2 { font-size: 17px; margin: 0 0 20px; font-weight: 650; }
.chapter-list { border-top: 1px solid var(--line); }
.chapter-row { display: flex; align-items: center; gap: 22px; padding: 22px 12px; border-bottom: 1px solid var(--line); text-decoration: none; color: inherit; transition: background .15s; border-radius: 4px; }
.chapter-row:hover { background: var(--soft); }
.chapter-number { font-family: monospace; color: var(--muted); font-size: 13px; flex-shrink: 0; }
.chapter-info { flex: 1; min-width: 0; }
h3 { font-size: 15px; line-height: 1.5; margin: 0; font-weight: 550; overflow-wrap: anywhere; }
.chapter-meta { display: flex; gap: 16px; color: var(--muted); font-size: 12px; margin-top: 7px; }
.access-label { display: inline-flex; align-items: center; gap: 5px; }
.chapter-arrow { color: var(--muted); flex-shrink: 0; }
.module-sidebar { padding: 0 0 0 24px; border-left: 1px solid var(--line); }
.module-sidebar h2 { font-size: 15px; }
.outcomes ul { list-style: none; padding: 0; margin: 0; display: grid; gap: 16px; }
.outcomes li { display: flex; align-items: flex-start; gap: 10px; font-size: 13px; line-height: 1.8; color: var(--muted); overflow-wrap: anywhere; }
.outcomes li svg { flex-shrink: 0; margin-top: 4px; color: var(--green); }
.prerequisites:not(:first-child) { margin-top: 32px; }
.prerequisite-chain { list-style: none; padding: 0; margin: 0; }
.prerequisite-chain li { position: relative; padding-bottom: 16px; }
.prerequisite-chain li:last-child { padding-bottom: 0; }
.prerequisite-chain li:not(:last-child)::after { content: ''; position: absolute; left: 12px; top: 27px; bottom: 3px; border-left: 1px solid var(--line); }
.prerequisite-chain a { display: flex; align-items: center; gap: 11px; text-decoration: none; color: var(--muted); font-size: 13px; line-height: 1.6; }
.prerequisite-chain a:hover { color: var(--green); }
.chain-number { display: grid; place-items: center; width: 25px; height: 25px; flex-shrink: 0; border: 1px solid var(--line); border-radius: 50%; font-size: 11px; font-family: monospace; }
.prerequisite-chain a > span:nth-child(2) { flex: 1; overflow-wrap: anywhere; }
.state-box { padding: 60px 0; color: var(--muted); }
.state-box a { color: var(--green); }
a:focus-visible { outline: 2px solid var(--green); outline-offset: 4px; }
@media (max-width: 800px) {
  .module-page { padding: 24px 20px 56px; }
  .crumb { margin-bottom: 32px; }
  .hero-section { flex-direction: column; align-items: flex-start; gap: 24px; }
  .module-content.with-outcomes { grid-template-columns: 1fr; gap: 32px; }
  .module-sidebar { padding: 24px 0 0; border-left: 0; border-top: 1px solid var(--line); }
}
</style>
