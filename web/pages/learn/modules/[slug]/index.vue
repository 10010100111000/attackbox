<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useFetch, useHead } from '#imports'

definePageMeta({
  layout: 'learning'
})

interface ChapterItem {
  id: string
  slug: string
  title: string
  access: 'public' | 'authenticated'
  position: number
  questionsCount: number
  isLocked: boolean
}

interface ModuleDetail {
  id: string
  slug: string
  title: string
  summary: string
  descriptionMarkdown: string
  difficulty: 'beginner' | 'intermediate' | 'advanced'
  coverAssetId: string | null
  publishedAt: string
  chapters: ChapterItem[]
}

const route = useRoute()
const slug = computed(() => String(route.params.slug || ''))

const { data: response, pending, error } = await useFetch<{ ok: boolean; data: ModuleDetail }>(
  () => `/api/modules/${slug.value}`
)

const moduleData = computed<ModuleDetail | null>(() => {
  return response.value?.data || null
})

const pageTitle = computed(() => {
  return moduleData.value ? `${moduleData.value.title} — AttackBox 学习路线` : '学习模块 — AttackBox'
})

useHead({
  title: pageTitle,
  meta: [
    {
      name: 'description',
      content: computed(() => moduleData.value?.summary || '循序渐进掌握核心技能。')
    }
  ]
})

function difficultyToLabel(diff?: string): string {
  switch (diff) {
    case 'beginner': return '基础入门'
    case 'intermediate': return '中级进阶'
    case 'advanced': return '高阶实战'
    default: return '学习模块'
  }
}

const firstChapterSlug = computed(() => {
  const chs = moduleData.value?.chapters || []
  return chs.length > 0 ? chs[0].slug : null
})
</script>

<template>
  <div class="roadmap-page">
    <div class="container">
      <!-- 顶部面包屑 -->
      <nav class="crumb" aria-label="面包屑">
        <NuxtLink to="/learn">学习空间</NuxtLink>
        <span>/</span>
        <span class="crumb-current">{{ moduleData?.title || slug }}</span>
      </nav>

      <!-- 加载中 -->
      <div v-if="pending" class="state-box">
        正在获取模块详情…
      </div>

      <!-- 404 或加载失败 -->
      <div v-else-if="error || !moduleData" class="state-box">
        <h2>模块不存在或尚未公开发布</h2>
        <p>该学习模块可能已被下架或尚未完成发布。</p>
        <NuxtLink to="/learn" class="back-home-btn">返回学习主页</NuxtLink>
      </div>

      <!-- 正常模块概览 -->
      <div v-else>
        <!-- 头部 Hero -->
        <section class="hero-section">
          <div class="hero-content">
            <div class="tag-row">
              <span class="tag difficulty-tag">{{ difficultyToLabel(moduleData.difficulty) }}</span>
              <span class="tag quiz-tag">章节测验</span>
            </div>
            <h1 class="hero-title">{{ moduleData.title }}</h1>
            <p class="hero-desc">{{ moduleData.summary || '建立扎实的基础认知与操作经验。' }}</p>

            <div class="stats-row">
              <span class="stat-item">◷ 约 {{ moduleData.chapters.length * 12 }} 分钟</span>
              <span class="stat-item">▤ {{ moduleData.chapters.length }} 个章节</span>
              <span class="stat-item">◎ 无需复杂前置</span>
            </div>

            <div class="instruction-line">
              <span class="check-icon">✓</span>
              <span>阅读教程 → 通过测验 → 解锁下一章</span>
            </div>
          </div>

          <!-- 右侧学习进度卡片 -->
          <aside class="summary-card">
            <div class="summary-eyebrow">YOUR LEARNING PATH</div>
            <div class="progress-header">
              <strong>学习进度</strong>
              <span class="progress-pct">0%</span>
            </div>
            <div class="progress-bar">
              <i style="width: 0%"></i>
            </div>
            <p class="progress-caption">共 {{ moduleData.chapters.length }} 个章节待完成</p>

            <NuxtLink
              v-if="firstChapterSlug"
              :to="`/learn/modules/${moduleData.slug}/${firstChapterSlug}`"
              class="start-learning-btn"
            >
              开始学习 →
            </NuxtLink>
          </aside>
        </section>

        <!-- 双栏路线布局 -->
        <div class="road-layout">
          <!-- 左侧章节路线 -->
          <section class="road-section">
            <div class="road-header">
              <h2 class="road-title">你的学习路线</h2>
              <span class="road-subtitle">循序渐进，一次一步</span>
            </div>

            <div class="road-list">
              <div
                v-for="(chapter, index) in moduleData.chapters"
                :key="chapter.id"
                class="road-item"
                :class="{ locked: chapter.isLocked }"
              >
                <div class="step-circle" :class="{ current: index === 0 && !chapter.isLocked }">
                  {{ index + 1 }}
                </div>

                <div class="chapter-info">
                  <h3 class="chapter-title">{{ chapter.title }}</h3>
                  <p class="chapter-meta">
                    {{ chapter.questionsCount }} 道练习题
                    <span v-if="chapter.access === 'authenticated'" class="auth-required-badge">· 需登录</span>
                  </p>
                </div>

                <div class="chapter-action">
                  <NuxtLink
                    v-if="!chapter.isLocked"
                    :to="`/learn/modules/${moduleData.slug}/${chapter.slug}`"
                    class="enter-chapter-link"
                  >
                    进入学习 →
                  </NuxtLink>
                  <span v-else class="locked-badge" title="受限章节，请登录账号后解锁">
                    🔒 待登录
                  </span>
                </div>
              </div>
            </div>
          </section>

          <!-- 右侧指引 -->
          <aside class="aside-guidance">
            <div class="guidance-block">
              <h3>学完后，你将能够</h3>
              <p>
                ✓ 掌握核心基础原理与常用命令<br />
                ✓ 独立完成实验环境中的排错与验证<br />
                ✓ 在真实场景中做出合理的安全决策
              </p>
            </div>

            <div class="guidance-block">
              <h3>关于章节测验</h3>
              <p>
                每章均配置标准练习题，全部答对即可解锁下一章。答错会获得排查提示，可随时重试。
              </p>
            </div>
          </aside>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.roadmap-page {
  padding: 32px 0 64px;
}

.container {
  max-width: 1190px;
  margin: 0 auto;
  padding: 0 32px;
}

/* 面包屑导航 */
.crumb {
  font-size: 13px;
  color: var(--muted);
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 28px;
}

.crumb a {
  color: var(--muted);
  text-decoration: none;
  transition: color 0.15s ease;
}

.crumb a:hover {
  color: var(--green);
}

.crumb-current {
  color: var(--ink);
  font-weight: 500;
}

/* Hero 头部 */
.hero-section {
  display: grid;
  grid-template-columns: 1fr 300px;
  gap: 60px;
  border-bottom: 1px solid var(--line);
  padding-bottom: 40px;
}

.hero-content {
  max-width: 680px;
}

.tag-row {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.tag {
  font-size: 11px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 4px;
  background: var(--mint);
  color: var(--green);
}

.quiz-tag {
  background: var(--soft);
  color: var(--muted);
}

.hero-title {
  font-size: 32px;
  font-weight: 750;
  letter-spacing: -0.6px;
  margin: 0 0 12px;
  color: var(--ink);
}

.hero-desc {
  font-size: 15px;
  color: var(--muted);
  line-height: 1.6;
  margin: 0 0 20px;
}

.stats-row {
  display: flex;
  gap: 20px;
  font-size: 13px;
  color: var(--muted);
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.instruction-line {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--ink);
}

.check-icon {
  color: var(--green);
  font-weight: bold;
}

/* 进度卡片 */
.summary-card {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 24px;
  align-self: start;
}

.summary-eyebrow {
  font-family: monospace;
  font-size: 10px;
  letter-spacing: 1.5px;
  color: var(--muted);
  margin-bottom: 12px;
}

.progress-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
  margin-bottom: 8px;
  color: var(--ink);
}

.progress-pct {
  font-size: 12px;
  color: var(--muted);
}

.progress-bar {
  height: 6px;
  background: var(--soft);
  border-radius: 6px;
  overflow: hidden;
  margin-bottom: 10px;
}

.progress-bar i {
  display: block;
  background: var(--green);
  height: 100%;
}

.progress-caption {
  font-size: 12px;
  color: var(--muted);
  margin: 0 0 20px;
}

.start-learning-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  padding: 10px;
  background: var(--green);
  color: var(--on-accent);
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  transition: opacity 0.15s ease;
}

.start-learning-btn:hover {
  opacity: 0.9;
}

/* 路线列表 */
.road-layout {
  display: grid;
  grid-template-columns: 1fr 300px;
  gap: 60px;
  padding-top: 36px;
}

.road-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 20px;
}

.road-title {
  font-size: 20px;
  font-weight: 700;
  margin: 0;
  color: var(--ink);
}

.road-subtitle {
  font-size: 12px;
  color: var(--muted);
}

.road-list {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 0 24px;
}

.road-item {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 22px 0;
  border-bottom: 1px solid var(--line);
}

.road-item:last-child {
  border-bottom: none;
}

.step-circle {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border-radius: 50%;
  border: 1px solid var(--line);
  display: grid;
  place-items: center;
  font-family: monospace;
  font-size: 13px;
  color: var(--muted);
}

.step-circle.current {
  background: var(--mint);
  border-color: var(--green);
  color: var(--green);
  font-weight: 700;
}

.chapter-info {
  flex: 1;
}

.chapter-title {
  font-size: 16px;
  font-weight: 650;
  margin: 0 0 4px;
  color: var(--ink);
}

.chapter-meta {
  font-size: 12px;
  color: var(--muted);
  margin: 0;
}

.auth-required-badge {
  color: var(--accent);
}

.enter-chapter-link {
  color: var(--green);
  font-weight: 600;
  font-size: 13px;
  text-decoration: none;
  white-space: nowrap;
}

.enter-chapter-link:hover {
  text-decoration: underline;
}

.locked-badge {
  font-size: 12px;
  color: var(--muted);
  white-space: nowrap;
}

/* 侧边指引 */
.aside-guidance {
  padding-top: 6px;
}

.guidance-block {
  margin-bottom: 32px;
}

.guidance-block h3 {
  font-size: 14px;
  font-weight: 650;
  color: var(--ink);
  margin: 0 0 10px;
}

.guidance-block p {
  font-size: 13px;
  color: var(--muted);
  line-height: 1.7;
  margin: 0;
}

/* 状态框 */
.state-box {
  text-align: center;
  padding: 64px 20px;
  background: var(--paper);
  border: 1px dashed var(--line);
  border-radius: 12px;
}

.state-box h2 {
  font-size: 18px;
  margin: 0 0 10px;
}

.state-box p {
  font-size: 14px;
  color: var(--muted);
  margin: 0 0 20px;
}

.back-home-btn {
  display: inline-block;
  padding: 8px 18px;
  background: var(--ink);
  color: var(--paper);
  border-radius: 6px;
  text-decoration: none;
  font-size: 13px;
}

@media (max-width: 840px) {
  .hero-section,
  .road-layout {
    grid-template-columns: 1fr;
    gap: 32px;
  }
}
</style>
