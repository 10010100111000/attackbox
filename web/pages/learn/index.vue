<script setup lang="ts">
import { ArrowRight, BookOpen } from 'lucide-vue-next'
import { useAuth } from '~/composables/useAuth'
import type { LearningHistoryItem } from '~/shared/learning'
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useFetch, useHead } from '#imports'
import type { ContentCategory } from '~/shared/content'

definePageMeta({
  layout: 'learning'
})

useHead({
  title: '学习 — AttackBox',
  meta: [
    { name: 'description', content: '选择一个模块开始，或沿着学习路线循序渐进。' }
  ]
})

interface ModuleItem {
  id: string
  slug: string
  title: string
  summary: string
  difficulty: 'beginner' | 'intermediate' | 'advanced'
  categoryId: string | null
  category: ContentCategory | null
  coverAssetId: string | null
  publishedAt: string
  chapterCount: number
}

const { currentUser } = useAuth()
const learnerId = computed(() => currentUser.value?.id || '')
const { data: historyResponse } = await useFetch<{ data: LearningHistoryItem[] }>('/api/me/learning', { key: 'learning:history', watch: [learnerId], immediate: !!learnerId.value })
const recentLearning = computed(() => learnerId.value ? historyResponse.value?.data?.find(item => !item.progress.isCompleted) || null : null)
const { data: response, pending, error, refresh } = await useFetch<{ ok: boolean; data: ModuleItem[] }>('/api/modules')

const modules = computed<ModuleItem[]>(() => {
  return response.value?.data || []
})

// 轮播内容数据 (完全忠于原型，去除多余验证性控制按钮)
const featureSlides = computed(() => {
  const picks = modules.value.slice(0, 3)
  return picks.length ? picks.map((module, index) => ({
    label: index === 0 ? '本期精选' : '模块推荐', title: module.title,
    description: module.summary, cta: '查看模块', href: `/learn/modules/${module.slug}`,
    command: '$ explore', caption: module.category?.name || 'AttackBox Learning',
  })) : [{ label: '开始探索', title: '从理解原理，到独立解决问题。', description: '浏览已发布的学习模块，选择自己的起点。', cta: '浏览学习模块', href: '#module-list', command: '$ explore', caption: 'AttackBox Learning' }]
})
watch(() => featureSlides.value.length, length => { if (carouselIndex.value >= length) carouselIndex.value = 0 })

const carouselIndex = ref(0)
const isPaused = ref(false)
let carouselTimer: ReturnType<typeof setInterval> | null = null

function nextSlide() {
  carouselIndex.value = (carouselIndex.value + 1) % featureSlides.value.length
}

function selectSlide(i: number) {
  carouselIndex.value = i
  restartTimer()
}

function startTimer() {
  if (carouselTimer) clearInterval(carouselTimer)
  carouselTimer = setInterval(() => {
    if (!isPaused.value) {
      nextSlide()
    }
  }, 6000)
}

function restartTimer() {
  if (carouselTimer) clearInterval(carouselTimer)
  startTimer()
}

onMounted(() => {
  startTimer()
})

onUnmounted(() => {
  if (carouselTimer) clearInterval(carouselTimer)
})

const activeTab = ref<'modules' | 'paths'>('modules')
const selectedCategory = ref('全部模块')
const searchQuery = ref('')

const { data: categoryResponse } = await useFetch<{ data: ContentCategory[] }>('/api/categories')
const categories = computed(() => ['全部模块', ...(categoryResponse.value?.data || []).map(category => category.name)])

function difficultyToLabel(diff: string): string {
  switch (diff) {
    case 'beginner': return '基础入门'
    case 'intermediate': return '中级进阶'
    case 'advanced': return '高阶实战'
    default: return '学习模块'
  }
}

const filteredModules = computed(() => {
  let list = modules.value

  if (selectedCategory.value !== '全部模块') {
    list = list.filter(m => {
      return m.category?.name === selectedCategory.value
    })
  }

  const query = searchQuery.value.trim().toLowerCase()
  if (query) {
    list = list.filter(m => {
      return (
        m.title.toLowerCase().includes(query) ||
        (m.summary && m.summary.toLowerCase().includes(query)) ||
        m.slug.toLowerCase().includes(query)
      )
    })
  }

  return list
})

// 预设优雅的生成式渐变主题与图案配置 (保留发散设计)
const artThemes = [
  {
    gradient: 'linear-gradient(135deg, rgba(20, 184, 166, 0.16) 0%, rgba(56, 189, 248, 0.12) 100%), radial-gradient(circle at 85% 15%, rgba(16, 185, 129, 0.22), transparent 60%)',
    border: 'rgba(20, 184, 166, 0.25)',
    type: 'radar'
  },
  {
    gradient: 'linear-gradient(135deg, rgba(99, 102, 241, 0.16) 0%, rgba(139, 92, 246, 0.12) 100%), radial-gradient(circle at 80% 20%, rgba(79, 70, 229, 0.22), transparent 60%)',
    border: 'rgba(99, 102, 241, 0.25)',
    type: 'grid'
  },
  {
    gradient: 'linear-gradient(135deg, rgba(245, 158, 11, 0.16) 0%, rgba(244, 63, 94, 0.12) 100%), radial-gradient(circle at 85% 20%, rgba(234, 88, 12, 0.2), transparent 60%)',
    border: 'rgba(245, 158, 11, 0.25)',
    type: 'waves'
  },
  {
    gradient: 'linear-gradient(135deg, rgba(168, 85, 247, 0.16) 0%, rgba(236, 72, 153, 0.12) 100%), radial-gradient(circle at 75% 25%, rgba(192, 38, 211, 0.2), transparent 60%)',
    border: 'rgba(168, 85, 247, 0.25)',
    type: 'nodes'
  },
  {
    gradient: 'linear-gradient(135deg, rgba(14, 165, 233, 0.16) 0%, rgba(45, 212, 191, 0.12) 100%), radial-gradient(circle at 80% 20%, rgba(6, 182, 212, 0.22), transparent 60%)',
    border: 'rgba(14, 165, 233, 0.25)',
    type: 'rings'
  }
]

function getModuleTheme(index: number) {
  return artThemes[index % artThemes.length]
}

</script>

<template>
  <div class="learn-page">
    <div class="container">
      <!-- 轮播 Banner (保留自动轮播与终端艺术卡片，去除验证性 carousel-controls) -->
      <section
        class="feature-banner feature-carousel"
        aria-roledescription="轮播"
        aria-label="推荐内容"
        @mouseenter="isPaused = true"
        @mouseleave="isPaused = false"
      >
        <div class="carousel-copy">
          <div class="feature-label">{{ featureSlides[carouselIndex].label }}</div>
          <h1>{{ featureSlides[carouselIndex].title }}</h1>
          <p>{{ featureSlides[carouselIndex].description }}</p>
          <NuxtLink :to="featureSlides[carouselIndex].href" class="btn">
            {{ featureSlides[carouselIndex].cta }}
          </NuxtLink>
        </div>

        <div class="banner-art" aria-hidden="true">
          <div class="terminal-art">
            <div class="dots"><i></i><i></i><i></i></div>
            <div class="cmd">{{ featureSlides[carouselIndex].command }}</div>
            <div class="dim">{{ featureSlides[carouselIndex].caption }}</div>
            <div class="cursor-line">▍</div>
          </div>
        </div>

        <!-- 轮播胶囊圆点指示器 (纯粹保留，无杂乱数字与暂停按钮) -->
        <div class="carousel-pagination" aria-label="选择推荐内容">
          <button
            v-for="(s, i) in featureSlides"
            :key="s.label"
            class="carousel-dot"
            :class="{ active: carouselIndex === i }"
            :aria-label="`第 ${i + 1} 项：${s.label}`"
            :aria-current="carouselIndex === i ? 'true' : undefined"
            type="button"
            @click="selectSlide(i)"
          >
            <span></span>
          </button>
        </div>
      </section>

      <NuxtLink v-if="recentLearning && activeTab === 'modules'" :to="`/learn/modules/${recentLearning.slug}/${recentLearning.progress.resumeChapterSlug}`" class="recent-learning">
        <div class="recent-copy"><span class="recent-label">继续学习</span><h2>{{ recentLearning.title }}</h2><p>{{ recentLearning.chapterTitle }}</p><div class="recent-progress" :aria-label="`已完成 ${recentLearning.progress.completedChapters} / ${recentLearning.progress.totalChapters} 章节`"><span :style="{ width: `${recentLearning.progress.percent}%` }" /></div><span class="recent-count">已完成 {{ recentLearning.progress.completedChapters }} / {{ recentLearning.progress.totalChapters }} 章节</span></div><ArrowRight :size="20" aria-hidden="true" />
      </NuxtLink>
      <!-- 二级标签栏 (学习模块 / 学习路线) -->
      <div class="page-tabs" role="tablist" aria-label="学习导航">
        <button
          type="button"
          class="tab-link"
          :class="{ selected: activeTab === 'modules' }"
          @click="activeTab = 'modules'"
        >
          学习模块
        </button>
        <button
          type="button"
          class="tab-link"
          :class="{ selected: activeTab === 'paths' }"
          @click="activeTab = 'paths'"
        >
          学习路线
        </button>
      </div>

      <!-- Tab 1: 学习模块 -->
      <div v-if="activeTab === 'modules'" id="module-list" class="tab-content">
        <!-- 筛选与搜索工具栏 -->
        <div class="toolbar">
          <div class="filters" role="tablist" aria-label="分类过滤">
            <button
              v-for="cat in categories"
              :key="cat"
              type="button"
              class="filter-btn"
              :class="{ active: selectedCategory === cat }"
              @click="selectedCategory = cat"
            >
              {{ cat }}
            </button>
          </div>
          <input
            v-model="searchQuery"
            type="search"
            class="search-input"
            placeholder="搜索模块"
            aria-label="搜索模块"
          />
        </div>

        <!-- 加载状态 -->
        <div v-if="pending" class="state-panel">
          正在加载模块列表…
        </div>

        <!-- 错误状态 -->
        <div v-else-if="error" class="state-panel error">
          <p>模块列表加载失败，请检查网络连接。</p>
          <button type="button" class="btn secondary" @click="() => refresh()">重新加载</button>
        </div>

        <!-- 卡片网格 (三列，生成式渐变与图形，克制箭头) -->
        <div v-else-if="filteredModules.length > 0" class="cards">
          <NuxtLink
            v-for="(mod, index) in filteredModules"
            :key="mod.id"
            :to="`/learn/modules/${mod.slug}`"
            class="card"
          >
            <!-- 生成式渐变艺术区与微几何图腾 (保留发散设计) -->
            <div
              class="card-art"
              :style="{ background: getModuleTheme(index).gradient }"
            >
              <!-- 几何 SVG 图腾 -->
              <svg class="art-svg" viewBox="0 0 100 60" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <!-- 节点网络 -->
                <template v-if="getModuleTheme(index).type === 'nodes'">
                  <circle cx="20" cy="30" r="3" fill="currentColor" fill-opacity="0.3" />
                  <circle cx="50" cy="18" r="4" fill="currentColor" fill-opacity="0.4" />
                  <circle cx="80" cy="36" r="3" fill="currentColor" fill-opacity="0.3" />
                  <circle cx="45" cy="46" r="2.5" fill="currentColor" fill-opacity="0.25" />
                  <line x1="20" y1="30" x2="50" y2="18" stroke="currentColor" stroke-opacity="0.25" stroke-dasharray="2 2" />
                  <line x1="50" y1="18" x2="80" y2="36" stroke="currentColor" stroke-opacity="0.25" />
                  <line x1="20" y1="30" x2="45" y2="46" stroke="currentColor" stroke-opacity="0.2" />
                </template>

                <!-- 雷达波同心弧 -->
                <template v-else-if="getModuleTheme(index).type === 'radar' || getModuleTheme(index).type === 'rings'">
                  <circle cx="85" cy="55" r="22" stroke="currentColor" stroke-opacity="0.2" stroke-width="1.2" />
                  <circle cx="85" cy="55" r="44" stroke="currentColor" stroke-opacity="0.15" stroke-dasharray="3 3" />
                  <circle cx="85" cy="55" r="66" stroke="currentColor" stroke-opacity="0.1" />
                  <line x1="85" y1="55" x2="45" y2="20" stroke="currentColor" stroke-opacity="0.2" />
                </template>

                <!-- 网格线框 -->
                <template v-else-if="getModuleTheme(index).type === 'grid'">
                  <path d="M10 15L90 15M10 30L90 30M10 45L90 45" stroke="currentColor" stroke-opacity="0.15" stroke-width="1" />
                  <path d="M25 5L25 55M50 5L50 55M75 5L75 55" stroke="currentColor" stroke-opacity="0.15" stroke-width="1" />
                  <circle cx="50" cy="30" r="3" fill="currentColor" fill-opacity="0.4" />
                </template>

                <!-- 拓扑等高波纹 -->
                <template v-else>
                  <path d="M10 45C30 35 45 50 65 38C75 32 85 40 90 35" stroke="currentColor" stroke-opacity="0.25" stroke-width="1.5" />
                  <path d="M10 30C28 20 48 35 68 22C78 16 85 24 90 20" stroke="currentColor" stroke-opacity="0.18" stroke-width="1.2" stroke-dasharray="2 2" />
                  <path d="M10 18C32 10 50 22 70 12C80 8 85 14 90 10" stroke="currentColor" stroke-opacity="0.1" stroke-width="1" />
                </template>
              </svg>

              <!-- 模块序号艺术字符 -->
              <div class="art-index-pill">
                {{ String(index + 1).padStart(2, '0') }}
              </div>
            </div>

            <!-- 卡片主体 -->
            <div class="card-body">
              <span class="tag">{{ difficultyToLabel(mod.difficulty) }}</span>
              <h2 class="card-title">{{ mod.title }}</h2>
              <p v-if="mod.summary" class="card-desc">{{ mod.summary }}</p>

              <div class="card-meta">
                <span>{{ mod.chapterCount }} 章节</span>
                <span v-if="mod.category">{{ mod.category.name }}</span>
              </div>

              <!-- 卡片底栏 (保持克制，去除多余 ↗ 箭头) -->
              <div class="card-bottom">
                <span class="status-action">查看模块</span>
                <span class="status-label">已开放</span>
              </div>
            </div>
          </NuxtLink>
        </div>

        <!-- 空搜索结果 -->
        <div v-else class="empty-notice">
          <p>没有匹配的模块，试试其他关键词或分类。</p>
        </div>
      </div>

      <!-- Tab 2: 学习路线 -->
      <div v-else class="tab-content">
        <div class="empty-notice"><BookOpen :size="24" aria-hidden="true" /><p>学习路线尚未开放，可先选择学习模块。</p><button type="button" class="btn secondary" @click="activeTab = 'modules'">浏览模块</button></div>
      </div>

      <!-- 底部理念标语 (与原型一致) -->
      <div class="footer-tagline">
        LEARN WITH INTENTION. BUILD WITH CONFIDENCE.
      </div>
    </div>
  </div>
</template>

<style scoped>
.recent-learning { display: flex; align-items: center; justify-content: space-between; gap: 24px; padding: 24px 28px; margin-bottom: 24px; border: 1px solid var(--line); border-radius: 10px; background: var(--mint); color: var(--ink); text-decoration: none; }
.recent-copy { flex: 1; min-width: 0; }
.recent-label { font-size: 11px; color: var(--muted); }
.recent-learning h2 { font-size: 20px; margin: 6px 0; font-weight: 650; }
.recent-learning p { font-size: 12px; color: var(--muted); margin: 0 0 14px; }
.recent-progress { height: 4px; width: min(100%, 320px); border-radius: 4px; background: var(--line); overflow: hidden; }
.recent-progress span { display: block; height: 100%; background: var(--green); }
.recent-count { display: block; margin-top: 8px; font-size: 11px; color: var(--muted); }

.learn-page {
  width: 100%;
  min-height: calc(100vh - 68px);
  background: var(--bg);
  color: var(--ink);
}

.container {
  max-width: 1100px;
  margin: 0 auto;
  padding: 34px 32px 64px;
}

/* 轮播 Banner (完全匹配原型，移除了验证性的 controls) */
.feature-banner {
  display: grid;
  grid-template-columns: 1fr 230px;
  gap: 25px;
  align-items: center;
  padding: 29px 32px;
  border: 1px solid var(--line);
  background: var(--paper);
  border-radius: 11px;
  min-height: 230px;
  position: relative;
  overflow: hidden;
  margin-bottom: 24px;
}

.carousel-copy {
  min-width: 0;
  z-index: 2;
}

.feature-label {
  color: var(--muted);
  font-size: 11px;
  margin-bottom: 10px;
  display: flex;
  align-items: center;
  gap: 9px;
  letter-spacing: 0.5px;
}

.feature-label::before {
  content: '';
  display: block;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--green);
}

.feature-banner h1 {
  font-size: 26px;
  margin: 0 0 8px;
  letter-spacing: -0.8px;
  color: var(--ink);
  line-height: 1.35;
}

.feature-banner p {
  font-size: 13px;
  margin: 0 0 18px;
  max-width: 540px;
  color: var(--muted);
  line-height: 1.6;
}

.feature-banner .btn {
  font-size: 12px;
  padding: 9px 18px;
  border-radius: 6px;
  background: var(--green);
  color: var(--on-accent);
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  font-weight: 550;
  transition: opacity 0.15s ease;
}

.feature-banner .btn:hover {
  opacity: 0.9;
}

/* 轮播右侧倾斜终端艺术卡片 */
.banner-art {
  height: 130px;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
}

.terminal-art {
  width: 200px;
  background: var(--code);
  color: var(--code-ink);
  border-radius: 8px;
  padding: 12px 16px;
  transform: rotate(-4deg);
  box-shadow: 9px 10px 0 var(--soft);
  font: 12px/2 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.terminal-art .dots {
  display: flex;
  gap: 4px;
  margin-bottom: 12px;
}

.terminal-art .dots i {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--code-muted);
  opacity: 0.65;
}

.terminal-art .cmd {
  color: var(--code-highlight);
}

.terminal-art .dim {
  color: var(--code-muted);
  font-size: 10px;
}

.cursor-line {
  margin-top: 5px;
  color: var(--green);
  animation: blink 1s step-start infinite;
}

@keyframes blink {
  50% { opacity: 0; }
}

/* 轮播分页胶囊指示器 (仅保留纯粹的分页点) */
.carousel-pagination {
  position: absolute;
  bottom: 14px;
  left: 28px;
  display: flex;
  align-items: center;
  gap: 4px;
  z-index: 5;
}

.carousel-dot {
  width: 26px;
  height: 26px;
  padding: 8px 4px;
  background: transparent;
  display: grid;
  place-items: center;
  border-radius: 5px;
  border: none;
  cursor: pointer;
}

.carousel-dot span {
  height: 4px;
  width: 6px;
  border-radius: 5px;
  background: var(--muted);
  opacity: 0.35;
  transition: width 0.2s ease, opacity 0.2s ease, background-color 0.2s ease;
}

.carousel-dot.active span {
  width: 18px;
  opacity: 1;
  background: var(--ink);
}

/* 二级 Tab 切换栏 */
.page-tabs {
  display: flex;
  gap: 28px;
  border-bottom: 1px solid var(--line);
  margin: 18px 0 24px;
}

.tab-link {
  font-size: 13px;
  padding: 0 0 12px;
  color: var(--muted);
  background: transparent;
  border: none;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: all 0.15s ease;
}

.tab-link:hover {
  color: var(--ink);
}

.tab-link.selected {
  color: var(--ink);
  border-bottom-color: var(--ink);
  font-weight: 600;
}

/* 工具栏 */
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 22px;
}

.filters {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-btn {
  padding: 6px 14px;
  border: 1px solid var(--line);
  border-radius: 7px;
  background: var(--paper);
  color: var(--muted);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.filter-btn:hover {
  color: var(--ink);
  border-color: var(--focus);
}

.filter-btn.active {
  background: var(--ink);
  color: var(--paper);
  border-color: var(--ink);
  font-weight: 550;
}

.search-input {
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 8px 14px;
  background: var(--paper);
  min-width: 220px;
  font-size: 12px;
  color: var(--ink);
  outline: none;
  transition: border-color 0.15s ease;
}

.search-input:focus {
  border-color: var(--focus);
}

/* 状态容器 */
.state-panel {
  padding: 40px;
  text-align: center;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 10px;
  color: var(--muted);
  font-size: 13px;
}

.empty-notice {
  padding: 40px;
  text-align: center;
  color: var(--muted);
  font-size: 13px;
}

/* 卡片网格 (3 列，克制优雅) */
.cards {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
}

.card {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
  overflow: hidden;
  text-decoration: none;
  color: var(--ink);
  display: flex;
  flex-direction: column;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}

.card:hover {
  transform: translateY(-2px);
  border-color: var(--focus);
  box-shadow: 0 10px 24px rgba(0, 0, 0, 0.05);
}

/* 专属艺术渐变区 */
.card-art {
  height: 108px;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  overflow: hidden;
  color: var(--ink);
}

.art-svg {
  position: absolute;
  right: 10px;
  bottom: 0;
  width: 140px;
  height: 80px;
  pointer-events: none;
  color: var(--ink);
}

.art-index-pill {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  color: var(--muted);
  border: 1px solid var(--line);
  background: var(--paper);
  padding: 2px 7px;
  border-radius: 5px;
  z-index: 2;
}

/* 卡片主体 */
.card-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  flex: 1;
}

.tag {
  align-self: flex-start;
  display: inline-block;
  border: 1px solid var(--line);
  border-radius: 5px;
  padding: 2px 8px;
  font-size: 11px;
  color: var(--muted);
  background: var(--soft);
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  margin: 10px 0 6px;
  color: var(--ink);
  letter-spacing: -0.3px;
}

.card-desc {
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted);
  margin: 0 0 16px;
  min-height: 38px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-meta {
  display: flex;
  gap: 12px;
  color: var(--muted);
  font-size: 11px;
  margin-top: auto;
  margin-bottom: 16px;
}

/* 卡片底栏 (保持克制，无冗余箭头) */
.card-bottom {
  border-top: 1px solid var(--line);
  padding-top: 14px;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.status-action {
  color: var(--ink);
  font-weight: 550;
  transition: color 0.15s ease;
}

.card:hover .status-action {
  color: var(--green);
}

.status-label {
  color: var(--muted);
  font-size: 11px;
}

/* Tab 2: 路线列表 */
.paths-list {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.path-card {
  max-width: 720px;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 11px;
  padding: 24px;
  text-decoration: none;
  color: var(--ink);
  display: block;
  transition: border-color 0.15s ease, transform 0.15s ease;
}

.path-card:hover {
  border-color: var(--focus);
  transform: translateY(-2px);
}

.path-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.path-title {
  font-size: 17px;
  font-weight: 650;
  margin: 0;
  color: var(--ink);
}

.path-action {
  font-size: 12px;
  color: var(--muted);
}

.path-card:hover .path-action {
  color: var(--ink);
}

.path-desc {
  font-size: 13px;
  color: var(--muted);
  margin: 0 0 20px;
  line-height: 1.6;
}

.mini-route {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.route-node {
  background: var(--soft);
  padding: 3px 10px;
  border-radius: 5px;
  color: var(--ink);
}

.route-divider {
  color: var(--muted);
}

.path-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-top: 1px solid var(--line);
  padding-top: 14px;
  font-size: 12px;
  color: var(--muted);
}

/* 底部标语 */
.footer-tagline {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 10px;
  letter-spacing: 1.5px;
  color: var(--muted);
  text-align: center;
  margin-top: 60px;
  opacity: 0.7;
}

/* 响应式调整 */
@media (max-width: 900px) {
  .feature-banner {
    grid-template-columns: 1fr;
    min-height: auto;
  }
  .banner-art {
    display: none;
  }
  .cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .container {
    padding: 20px 16px 48px;
  }
  .cards {
    grid-template-columns: 1fr;
  }
  .toolbar {
    flex-direction: column;
    align-items: stretch;
  }
  .search-input {
    width: 100%;
  }
}
</style>
