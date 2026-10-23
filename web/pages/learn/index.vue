<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
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

const { data: response, pending, error, refresh } = await useFetch<{ ok: boolean; data: ModuleItem[] }>('/api/modules')

const modules = computed<ModuleItem[]>(() => {
  return response.value?.data || []
})

// 轮播内容数据 (完全忠于原型，去除多余验证性控制按钮)
const featureSlides = [
  {
    label: '本期精选',
    title: '从理解原理，到独立解决问题。',
    description: '先建立基础，再进入靶场。挑选适合自己的起点，按自己的节奏探索。',
    cta: '探索入门路线',
    href: '/learn/modules/linux-basics',
    command: '$ start learning',
    caption: 'read → practice → understand'
  },
  {
    label: '动手挑战',
    title: '把刚学会的知识，带进靶场。',
    description: '从 Linux 权限到 Web 安全，在独立的练习环境中验证你的理解。',
    cta: '浏览挑战',
    href: '/challenges',
    command: '$ enter challenge',
    caption: 'explore → test → discover'
  },
  {
    label: '模块推荐',
    title: '从第一条 Linux 命令开始。',
    description: '认识 Shell、探索目录、理解权限，用三个章节打好命令行基础。',
    cta: '查看 Linux 入门',
    href: '/learn/modules/linux-basics',
    command: '$ pwd',
    caption: '/home/box'
  }
]

const carouselIndex = ref(0)
const isPaused = ref(false)
let carouselTimer: ReturnType<typeof setInterval> | null = null

function nextSlide() {
  carouselIndex.value = (carouselIndex.value + 1) % featureSlides.length
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

// 模拟路线数据（严格还原原型结构）
const pathsList = [
  {
    id: 'linux-security-basics',
    title: 'Linux 与安全基础',
    desc: '从命令行操作到 Web 安全，建立完整的入门知识体系。',
    nodes: ['Linux 入门', '网络基础', 'Web 安全基础'],
    meta: '3 个模块 · 入门',
    status: '0 / 3 模块已完成',
    targetSlug: 'linux-basics'
  }
]
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
      <div v-if="activeTab === 'modules'" class="tab-content">
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
              <p class="card-desc">{{ mod.summary || '包含原理讲解与交互式实操测验。' }}</p>

              <div class="card-meta">
                <span>{{ mod.chapterCount }} 章节</span>
                <span>约 {{ Math.max(15, mod.chapterCount * 12) }} 分钟</span>
                <span>含交互练习</span>
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
        <div class="paths-list">
          <NuxtLink
            v-for="p in pathsList"
            :key="p.id"
            :to="`/learn/modules/${p.targetSlug}`"
            class="path-card"
          >
            <div class="path-header">
              <h2 class="path-title">{{ p.title }}</h2>
              <span class="path-action">查看路线</span>
            </div>
            <p class="path-desc">{{ p.desc }}</p>

            <!-- 路线拓扑节点 -->
            <div class="mini-route" aria-label="路线包含的模块">
              <template v-for="(node, i) in p.nodes" :key="node">
                <span class="route-node">{{ node }}</span>
                <span v-if="i < p.nodes.length - 1" class="route-divider">→</span>
              </template>
            </div>

            <div class="path-footer">
              <span>{{ p.meta }}</span>
              <span>{{ p.status }}</span>
            </div>
          </NuxtLink>
        </div>
      </div>

      <!-- 底部理念标语 (与原型一致) -->
      <div class="footer-tagline">
        LEARN WITH INTENTION. BUILD WITH CONFIDENCE.
      </div>
    </div>
  </div>
</template>

<style scoped>
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
