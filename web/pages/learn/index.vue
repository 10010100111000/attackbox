<script setup lang="ts">
import { ArrowRight, BookOpen } from 'lucide-vue-next'
import { useAuth } from '~/composables/useAuth'
import type { LearningHistoryItem } from '~/shared/learning'
import { ref, computed, watch } from 'vue'
import { useLazyFetch, useHead } from '#imports'
import type { PublicModule } from '~/shared/module-catalog'

definePageMeta({
  layout: 'learning'
})

useHead({
  title: '学习 — AttackBox',
  meta: [{ name: 'description', content: '选择一个模块开始，或沿着学习路线循序渐进。' }]
})

const {
  data: routeResponse,
  pending: routesPending,
  error: routesError,
  refresh: refreshRoutes
} = useCatalogData<{ data: { id: string; title: string; summary: string; moduleCount: number }[] }>(
  'learning:catalog:routes',
  '/api/routes'
)
const { currentUser } = useAuth()
const learnerId = computed(() => currentUser.value?.id || '')
const { data: historyResponse } = useLazyFetch<{ data: LearningHistoryItem[] }>(
  '/api/me/learning',
  { key: 'learning:history', watch: [learnerId], immediate: !!learnerId.value }
)
const recentLearning = computed(() =>
  learnerId.value
    ? historyResponse.value?.data?.find(item => !item.progress.isCompleted) || null
    : null
)
const {
  data: response,
  pending,
  error,
  refresh
} = useCatalogData<{ ok: boolean; data: PublicModule[] }>(
  'learning:catalog:modules',
  '/api/modules'
)

const modules = computed<PublicModule[]>(() => {
  return response.value?.data || []
})

const { data: homeResponse, pending: homePending } = useCatalogData<{
  data: Omit<import('~/shared/learn-home').LearnHome, 'lockVersion'>
}>('learning:catalog:home', '/api/learn-home')

const activeTab = ref<'modules' | 'paths'>(useRoute().query.tab === 'routes' ? 'paths' : 'modules')
const selectedCategory = ref('全部模块')
const searchQuery = ref('')
const difficulty = ref('')
const sortBy = ref('published')
const difficultyFilterOptions = [
  { value: '', label: '全部难度' },
  { value: 'beginner', label: '基础入门' },
  { value: 'intermediate', label: '中级进阶' },
  { value: 'advanced', label: '高阶实战' }
]
const sortOptions = [
  { value: 'published', label: '最新发布' },
  { value: 'updated', label: '最近更新' }
]
const historyByModule = computed(
  () =>
    new Map(
      (learnerId.value ? historyResponse.value?.data || [] : []).map(item => [
        item.moduleId,
        item.progress
      ])
    )
)
const failedCovers = ref(new Set<string>())

const categories = computed(() => [
  '全部模块',
  ...new Set(modules.value.flatMap(m => (m.category ? [m.category.name] : [])))
])
watch(categories, values => {
  if (!values.includes(selectedCategory.value)) selectedCategory.value = '全部模块'
})
const hasFilters = computed(
  () => !!searchQuery.value || !!difficulty.value || selectedCategory.value !== '全部模块'
)
function resetFilters() {
  searchQuery.value = ''
  difficulty.value = ''
  selectedCategory.value = '全部模块'
}

function difficultyToLabel(diff: string): string {
  switch (diff) {
    case 'beginner':
      return '基础入门'
    case 'intermediate':
      return '中级进阶'
    case 'advanced':
      return '高阶实战'
    default:
      return '学习模块'
  }
}

const filteredModules = computed(() => {
  let list = modules.value

  if (selectedCategory.value !== '全部模块') {
    list = list.filter(m => {
      return m.category?.name === selectedCategory.value
    })
  }

  if (difficulty.value) list = list.filter(m => m.difficulty === difficulty.value)
  list = [...list].sort(
    (a, b) =>
      (sortBy.value === 'updated'
        ? b.updatedAt.localeCompare(a.updatedAt)
        : (b.publishedAt ?? '').localeCompare(a.publishedAt ?? '')) || a.id.localeCompare(b.id)
  )
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
</script>

<template>
  <div class="learn-page">
    <div class="learning-frame learning-shell learn-container">
      <div v-if="homePending && !homeResponse" class="home-placeholder" aria-hidden="true" />
      <LearningHomeCarousel
        v-if="homeResponse?.data.enabled"
        :slides="homeResponse.data.slides"
        :autoplay="homeResponse.data.autoplay"
        :interval-seconds="homeResponse.data.intervalSeconds"
      />

      <NuxtLink
        v-if="recentLearning && activeTab === 'modules'"
        :to="`/learn/modules/${recentLearning.slug}/${recentLearning.progress.resumeChapterSlug}`"
        class="recent-learning"
      >
        <div class="recent-copy">
          <span class="recent-label">继续学习</span>
          <h2>{{ recentLearning.title }}</h2>
          <p>{{ recentLearning.chapterTitle }}</p>
          <div
            class="recent-progress"
            :aria-label="`已完成 ${recentLearning.progress.completedChapters} / ${recentLearning.progress.totalChapters} 章节`"
          >
            <span :style="{ width: `${recentLearning.progress.percent}%` }" />
          </div>
          <span class="recent-count"
            >已完成 {{ recentLearning.progress.completedChapters }} /
            {{ recentLearning.progress.totalChapters }} 章节</span
          >
        </div>
        <span class="resume-action">继续学习</span>
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
        <!-- 筛选与搜索工具栏 (下沉凹槽轨道设计) -->
        <div class="toolbar">
          <div class="filters" role="group" aria-label="分类过滤">
            <button
              v-for="cat in categories"
              :key="cat"
              type="button"
              class="filter-btn"
              :class="{ active: selectedCategory === cat }"
              :aria-pressed="selectedCategory === cat"
              @click="selectedCategory = cat"
            >
              {{ cat }}
            </button>
          </div>
          <UiInput
            v-model="searchQuery"
            bare
            size="sm"
            type="search"
            class="search-input"
            placeholder="搜索模块"
            aria-label="搜索模块"
          />
        </div>

        <div class="discovery-controls">
          <span>{{ filteredModules.length }} 个模块</span
          ><label
            >难度<UiSelect
              v-model="difficulty"
              aria-label="筛选难度"
              :options="difficultyFilterOptions" /></label
          ><label
            >排序<UiSelect v-model="sortBy" aria-label="模块排序" :options="sortOptions"
          /></label>
        </div>
        <AdvertisingSlot placement="learn_catalog" />

        <!-- 加载状态 -->
        <LearningCatalogSkeleton v-if="pending && !response" label="正在读取学习模块" />

        <!-- 错误状态 -->
        <div v-else-if="error" class="state-panel error">
          <p>模块列表加载失败，请检查网络连接。</p>
          <button type="button" class="btn secondary" @click="() => refresh()">重新加载</button>
        </div>

        <!-- 卡片网格 -->
        <div v-else-if="filteredModules.length > 0" class="cards">
          <NuxtLink
            v-for="(mod, index) in filteredModules"
            :key="mod.id"
            :to="
              historyByModule.get(mod.id)?.resumeChapterSlug &&
              !historyByModule.get(mod.id)?.isCompleted
                ? `/learn/modules/${mod.slug}/${historyByModule.get(mod.id)?.resumeChapterSlug}`
                : `/learn/modules/${mod.slug}`
            "
            class="card"
          >
            <div class="card-art">
              <img
                v-if="mod.coverAssetId && !failedCovers.has(mod.id)"
                :src="`/api/modules/${mod.slug}/assets/${mod.coverAssetId}`"
                alt=""
                class="module-cover"
                loading="lazy"
                @error="failedCovers.add(mod.id)"
              />
              <LearningModuleCardArt
                :index="index"
                :has-cover="Boolean(mod.coverAssetId && !failedCovers.has(mod.id))"
              />
            </div>

            <div class="card-body">
              <span class="tag" :class="mod.difficulty">{{
                difficultyToLabel(mod.difficulty)
              }}</span>
              <h2 class="card-title">{{ mod.title }}</h2>
              <p v-if="mod.summary" class="card-desc">{{ mod.summary }}</p>

              <div class="card-meta">
                <span>{{ mod.chapterCount }} 章节</span>
                <span v-if="mod.category">{{ mod.category.name }}</span
                ><span v-if="mod.durationMinutes">约 {{ mod.durationMinutes }} 分钟</span>
              </div>

              <div class="card-bottom">
                <span class="status-action">{{
                  historyByModule.get(mod.id)?.isCompleted
                    ? '再次阅读'
                    : historyByModule.has(mod.id)
                      ? '继续学习'
                      : '查看模块'
                }}</span
                ><span v-if="historyByModule.has(mod.id)" class="status-label"
                  >{{ historyByModule.get(mod.id)?.percent }}%</span
                >
              </div>
            </div>
          </NuxtLink>
        </div>

        <!-- 空搜索结果 -->
        <div v-else class="empty-notice">
          <BookOpen :size="28" class="empty-doodle" aria-hidden="true" />
          <p>
            {{
              hasFilters
                ? '没找到合适的模块，换个方向试试？'
                : '这里还很安静。模块发布后，就会出现在这里。'
            }}
          </p>
          <button v-if="hasFilters" type="button" class="filter-btn" @click="resetFilters">
            清除筛选
          </button>
        </div>
      </div>

      <!-- Tab 2: 学习路线 -->
      <div v-else class="tab-content">
        <LearningCatalogSkeleton v-if="routesPending && !routeResponse" label="正在读取学习路线" />
        <div v-else-if="routesError" class="state-panel error">
          <p>学习路线加载失败，请稍后重试。</p>
          <button type="button" class="btn secondary" @click="() => refreshRoutes()">
            重新加载
          </button>
        </div>
        <div v-else-if="routeResponse?.data.length" class="published-routes">
          <NuxtLink
            v-for="item in routeResponse.data"
            :key="item.id"
            :to="`/learn/routes/${item.id}`"
            class="published-route"
            ><span>{{ item.moduleCount }} 个模块</span>
            <h2>{{ item.title }}</h2>
            <p>{{ item.summary }}</p>
            <ArrowRight :size="18" aria-hidden="true"
          /></NuxtLink>
        </div>
        <div v-else class="empty-notice">
          <BookOpen :size="24" aria-hidden="true" />
          <p>暂时没有已发布的路线。</p>
        </div>
      </div>

      <footer class="footer-tagline">LEARN WITH INTENTION. BUILD WITH CONFIDENCE.</footer>
    </div>
  </div>
</template>

<style scoped>
.home-placeholder {
  height: 176px;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  border-radius: 13px;
  margin-bottom: 20px;
}

.empty-doodle {
  color: var(--learning-spark);
  transform: rotate(-8deg);
  margin: 0 auto 12px;
}

.module-cover {
  width: 100%;
  height: 100%;
  object-fit: cover;
  position: absolute;
  inset: 0;
}

.discovery-controls {
  display: flex;
  align-items: center;
  gap: 20px;
  margin: -2px 0 20px;
  color: var(--muted);
  font-size: 12px;
}

.discovery-controls > span {
  margin-right: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11.5px;
  letter-spacing: 0.3px;
}

.discovery-controls label {
  display: flex;
  align-items: center;
  gap: 8px;
}

.recent-learning {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 18px 24px;
  margin-bottom: 22px;
  border: 1px solid var(--brand-border);
  border-radius: 13px;
  background:
    radial-gradient(ellipse 60% 120% at 0% 50%, var(--brand-soft), transparent 70%),
    var(--card-surface);
  box-shadow: var(--surface-highlight), var(--card-shadow);
  color: var(--ink);
  text-decoration: none;
  overflow: hidden;
  transition:
    transform 0.2s cubic-bezier(0.22, 1, 0.36, 1),
    border-color 0.2s ease,
    box-shadow 0.2s ease;
}

.recent-learning::before {
  content: '';
  position: absolute;
  left: 0;
  top: 14px;
  bottom: 14px;
  width: 3px;
  border-radius: 0 3px 3px 0;
  background: var(--brand);
}

.recent-learning:hover {
  transform: translateY(-2px);
  border-color: var(--brand);
  box-shadow: var(--card-hover-shadow);
}

.resume-action {
  padding: 8px 15px;
  border-radius: 7px;
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  font-size: 12.5px;
  font-weight: 650;
  box-shadow: var(--btn-primary-shadow);
  flex-shrink: 0;
}

.recent-copy {
  flex: 1;
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 150px;
  column-gap: 20px;
}

.recent-label {
  grid-column: 1;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.6px;
  color: var(--brand);
}

.recent-learning h2 {
  grid-column: 1;
  font-size: 16px;
  margin: 4px 0 3px;
  font-weight: 650;
  letter-spacing: -0.3px;
}

.recent-learning p {
  grid-column: 1;
  font-size: 12.5px;
  color: var(--muted);
  margin: 0 0 6px;
}

.recent-progress {
  grid-column: 2;
  grid-row: 2;
  align-self: center;
  height: 5px;
  width: min(100%, 320px);
  border-radius: 999px;
  background: var(--surface-well);
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.5);
  overflow: hidden;
}

.recent-progress span {
  display: block;
  height: 100%;
  background: var(--brand);
  border-radius: inherit;
}

.recent-count {
  grid-column: 2;
  grid-row: 3;
  display: block;
  margin-top: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  color: var(--muted);
}

.learn-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  width: 100%;
  min-height: calc(100dvh - var(--site-header-height));
  background: transparent;
  color: var(--ink);
}

.learn-container {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.page-tabs {
  display: flex;
  gap: 28px;
  border-bottom: 1px solid var(--line);
  margin: 18px 0 24px;
}

.tab-link {
  font-size: 13.5px;
  font-weight: 500;
  padding: 0 0 12px;
  color: var(--muted);
  background: transparent;
  border: none;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition:
    color 0.16s ease,
    border-color 0.16s ease;
}

.tab-link:hover {
  color: var(--ink);
}

.tab-link.selected {
  color: var(--ink);
  border-bottom-color: var(--brand);
  font-weight: 650;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 20px;
}

.filters {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  padding: 4px;
  border-radius: 10px;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  box-shadow: var(--well-shadow);
}

.filter-btn {
  padding: 6px 14px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition:
    color 0.16s ease,
    background 0.16s ease,
    box-shadow 0.16s ease;
}

.filter-btn:hover {
  color: var(--ink);
}

.filter-btn.active {
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  border-color: transparent;
  box-shadow: var(--btn-primary-shadow);
  font-weight: 650;
}

.search-input {
  min-width: 230px;
}

.state-panel {
  padding: 44px;
  text-align: center;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  box-shadow: var(--well-shadow);
  border-radius: 12px;
  color: var(--muted);
  font-size: 13px;
}

.empty-notice {
  padding: 56px 32px;
  text-align: center;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  box-shadow: var(--well-shadow);
  border-radius: 13px;
  color: var(--muted);
  font-size: 13px;
}

.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(288px, 1fr));
  gap: 20px;
}

.card {
  background: var(--card-surface);
  border: 1px solid var(--line);
  border-radius: 13px;
  box-shadow: var(--surface-highlight), var(--card-shadow);
  overflow: hidden;
  text-decoration: none;
  color: var(--ink);
  display: flex;
  flex-direction: column;
  animation: surface-rise 0.28s cubic-bezier(0.22, 1, 0.36, 1) both;
  transition:
    transform 0.24s cubic-bezier(0.22, 1, 0.36, 1),
    box-shadow 0.24s ease,
    border-color 0.24s ease;
}

.card:hover {
  transform: translateY(-4px);
  border-color: var(--brand-border);
  box-shadow: var(--card-hover-shadow);
}

.card-art {
  height: 112px;
  position: relative;
  overflow: hidden;
  color: var(--ink);
  background: color-mix(in srgb, var(--bg) 55%, var(--paper));
  border-bottom: 1px solid var(--line);
}

.card-body {
  padding: 20px 22px;
  display: flex;
  flex-direction: column;
  flex: 1;
}

.tag {
  align-self: flex-start;
  display: inline-flex;
  align-items: center;
  border: 1px solid transparent;
  border-radius: 5px;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  background: var(--soft);
}

.tag.beginner {
  color: var(--tone-green);
  background: var(--tone-green-soft);
  border-color: color-mix(in srgb, var(--tone-green) 24%, transparent);
}

.tag.intermediate {
  color: var(--tone-blue);
  background: var(--tone-blue-soft);
  border-color: color-mix(in srgb, var(--tone-blue) 24%, transparent);
}

.tag.advanced {
  color: var(--tone-red);
  background: var(--tone-red-soft);
  border-color: color-mix(in srgb, var(--tone-red) 24%, transparent);
}

.card-title {
  font-size: 16.5px;
  font-weight: 650;
  margin: 11px 0 6px;
  color: var(--ink);
  letter-spacing: -0.35px;
}

.card-desc {
  font-size: 12.5px;
  line-height: 1.65;
  color: var(--muted);
  margin: 0 0 18px;
  min-height: 40px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-meta {
  display: flex;
  gap: 12px;
  color: var(--muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  margin-top: auto;
  margin-bottom: 16px;
}

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
  font-weight: 600;
  transition: color 0.16s ease;
}

.card:hover .status-action {
  color: var(--brand);
}

.status-label {
  color: var(--muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
}

.published-routes {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(288px, 1fr));
  gap: 20px;
}

.published-route {
  position: relative;
  display: block;
  padding: 28px;
  background: var(--card-surface);
  border: 1px solid var(--line);
  border-radius: 13px;
  box-shadow: var(--surface-highlight), var(--card-shadow);
  color: var(--ink);
  text-decoration: none;
  transition:
    border-color 0.22s ease,
    transform 0.22s cubic-bezier(0.22, 1, 0.36, 1),
    box-shadow 0.22s ease;
}

.published-route:hover {
  border-color: var(--brand-border);
  transform: translateY(-3px);
  box-shadow: var(--card-hover-shadow);
}

.published-route span {
  color: var(--brand);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11.5px;
  font-weight: 600;
}

.published-route > svg {
  position: absolute;
  right: 28px;
  top: 28px;
  color: var(--muted);
  transition:
    transform 0.18s ease,
    color 0.18s ease;
}

.published-route:hover > svg {
  transform: translateX(3px);
  color: var(--brand);
}

.published-route h2 {
  overflow-wrap: anywhere;
  font-size: 21px;
  font-weight: 650;
  letter-spacing: -0.4px;
  margin: 14px 0 10px;
}

.published-route p {
  font-size: 13px;
  line-height: 1.7;
  color: var(--muted);
  margin: 0;
}

.footer-tagline {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 10px;
  letter-spacing: 1.6px;
  color: var(--muted);
  text-align: center;
  margin-top: auto;
  padding-top: 52px;
  opacity: 0.65;
}

@media (max-width: 900px) {
  .cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .discovery-controls {
    flex-wrap: wrap;
    gap: 10px;
  }
  .recent-copy {
    display: block;
  }
  .recent-progress {
    margin-top: 8px;
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
