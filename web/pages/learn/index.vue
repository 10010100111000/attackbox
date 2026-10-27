<script setup lang="ts">
import { ArrowRight, BookOpen } from 'lucide-vue-next'
import { useAuth } from '~/composables/useAuth'
import type { LearningHistoryItem } from '~/shared/learning'
import { ref, computed, watch } from 'vue'
import { useLazyFetch, useHead } from '#imports'
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
  updatedAt: string
  durationMinutes: number
  publishedAt: string
  chapterCount: number
}

const { data: routeResponse, pending: routesPending, error: routesError, refresh: refreshRoutes } = useCatalogData<{data:{id:string;title:string;summary:string;moduleCount:number}[]}>('learning:catalog:routes', '/api/routes')
const { currentUser } = useAuth()
const learnerId = computed(() => currentUser.value?.id || '')
const { data: historyResponse } = useLazyFetch<{ data: LearningHistoryItem[] }>('/api/me/learning', { key: 'learning:history', watch: [learnerId], immediate: !!learnerId.value })
const recentLearning = computed(() => learnerId.value ? historyResponse.value?.data?.find(item => !item.progress.isCompleted) || null : null)
const { data: response, pending, error, refresh } = useCatalogData<{ ok: boolean; data: ModuleItem[] }>('learning:catalog:modules', '/api/modules')

const modules = computed<ModuleItem[]>(() => {
  return response.value?.data || []
})

const { data: homeResponse, pending: homePending } = useCatalogData<{ data: Omit<import('~/shared/learn-home').LearnHome,'lockVersion'> }>('learning:catalog:home', '/api/learn-home')

const activeTab = ref<'modules' | 'paths'>(useRoute().query.tab === 'routes' ? 'paths' : 'modules')
const selectedCategory = ref('全部模块')
const searchQuery = ref('')
const difficulty = ref('')
const sortBy = ref('published')
const historyByModule = computed(() => new Map((learnerId.value ? historyResponse.value?.data || [] : []).map(item => [item.moduleId, item.progress])))
const failedCovers = ref(new Set<string>())

const categories = computed(() => ['全部模块', ...new Set(modules.value.flatMap(m => m.category ? [m.category.name] : []))])
watch(categories, values => { if (!values.includes(selectedCategory.value)) selectedCategory.value = '全部模块' })
const hasFilters = computed(() => !!searchQuery.value || !!difficulty.value || selectedCategory.value !== '全部模块')
function resetFilters() { searchQuery.value = ''; difficulty.value = ''; selectedCategory.value = '全部模块' }

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

  if (difficulty.value) list = list.filter(m => m.difficulty === difficulty.value)
  list = [...list].sort((a,b) => (sortBy.value === 'updated' ? b.updatedAt.localeCompare(a.updatedAt) : b.publishedAt.localeCompare(a.publishedAt)) || a.id.localeCompare(b.id))
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
    gradient: 'linear-gradient(135deg, rgba(255, 175, 136, 0.24) 0%, rgba(239, 124, 141, 0.18) 100%), radial-gradient(circle at 85% 15%, rgba(255, 224, 176, 0.25), transparent 60%)',
    border: 'rgba(255, 175, 136, 0.35)',
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
    <div class="learning-shell learn-container">
      <div v-if="homePending && !homeResponse" class="home-placeholder" aria-hidden="true"/>
      <LearningHomeCarousel v-if="homeResponse?.data.enabled" :slides="homeResponse.data.slides" :autoplay="homeResponse.data.autoplay" :interval-seconds="homeResponse.data.intervalSeconds" />

      <NuxtLink v-if="recentLearning && activeTab === 'modules'" :to="`/learn/modules/${recentLearning.slug}/${recentLearning.progress.resumeChapterSlug}`" class="recent-learning">
        <div class="recent-copy"><span class="recent-label">继续学习</span><h2>{{ recentLearning.title }}</h2><p>{{ recentLearning.chapterTitle }}</p><div class="recent-progress" :aria-label="`已完成 ${recentLearning.progress.completedChapters} / ${recentLearning.progress.totalChapters} 章节`"><span :style="{ width: `${recentLearning.progress.percent}%` }" /></div><span class="recent-count">已完成 {{ recentLearning.progress.completedChapters }} / {{ recentLearning.progress.totalChapters }} 章节</span></div><span class="resume-action">继续学习</span>
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
          <input
            v-model="searchQuery"
            type="search"
            class="search-input"
            placeholder="搜索模块"
            aria-label="搜索模块"
          />
        </div>

        <div class="discovery-controls"><span>{{ filteredModules.length }} 个模块</span><label>难度<select v-model="difficulty" aria-label="筛选难度"><option value="">全部难度</option><option value="beginner">基础入门</option><option value="intermediate">中级进阶</option><option value="advanced">高阶实战</option></select></label><label>排序<select v-model="sortBy" aria-label="模块排序"><option value="published">最新发布</option><option value="updated">最近更新</option></select></label></div>
        <!-- 加载状态 -->
        <LearningCatalogSkeleton v-if="pending && !response" label="正在读取学习模块" />

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
            :to="historyByModule.get(mod.id)?.resumeChapterSlug && !historyByModule.get(mod.id)?.isCompleted ? `/learn/modules/${mod.slug}/${historyByModule.get(mod.id)?.resumeChapterSlug}` : `/learn/modules/${mod.slug}`"
            class="card"
          >
            <!-- 生成式渐变艺术区与微几何图腾 (保留发散设计) -->
            <div
              class="card-art"
              :style="{ background: getModuleTheme(index).gradient }"
            >
              <!-- 几何 SVG 图腾 -->
              <img v-if="mod.coverAssetId && !failedCovers.has(mod.id)" :src="`/api/modules/${mod.slug}/assets/${mod.coverAssetId}`" alt="" class="module-cover" loading="lazy" @error="failedCovers.add(mod.id)"/><svg v-else class="art-svg" viewBox="0 0 100 60" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
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
              <span class="tag" :class="mod.difficulty">{{ difficultyToLabel(mod.difficulty) }}</span>
              <h2 class="card-title">{{ mod.title }}</h2>
              <p v-if="mod.summary" class="card-desc">{{ mod.summary }}</p>

              <div class="card-meta">
                <span>{{ mod.chapterCount }} 章节</span>
                <span v-if="mod.category">{{ mod.category.name }}</span><span v-if="mod.durationMinutes">约 {{ mod.durationMinutes }} 分钟</span>
              </div>

              <!-- 卡片底栏 (保持克制，去除多余 ↗ 箭头) -->
              <div class="card-bottom">
                <span class="status-action">{{ historyByModule.get(mod.id)?.isCompleted ? '再次阅读' : historyByModule.has(mod.id) ? '继续学习' : '查看模块' }}</span><span v-if="historyByModule.has(mod.id)" class="status-label">{{ historyByModule.get(mod.id)?.percent }}%</span>
              </div>
            </div>
          </NuxtLink>
        </div>

        <!-- 空搜索结果 -->
        <div v-else class="empty-notice">
          <BookOpen :size="28" class="empty-doodle" aria-hidden="true"/><p>{{ hasFilters ? '没找到合适的模块，换个方向试试？' : '这里还很安静。模块发布后，就会出现在这里。' }}</p><button v-if="hasFilters" type="button" class="filter-btn" @click="resetFilters">清除筛选</button>
        </div>
      </div>

      <!-- Tab 2: 学习路线 -->
      <div v-else class="tab-content">
        <LearningCatalogSkeleton v-if="routesPending && !routeResponse" label="正在读取学习路线" />
        <div v-else-if="routesError" class="state-panel error">
          <p>学习路线加载失败，请稍后重试。</p>
          <button type="button" class="btn secondary" @click="() => refreshRoutes()">重新加载</button>
        </div>
        <div v-else-if="routeResponse?.data.length" class="published-routes"><NuxtLink v-for="item in routeResponse.data" :key="item.id" :to="`/learn/routes/${item.id}`" class="published-route"><span>{{item.moduleCount}} 个模块</span><h2>{{item.title}}</h2><p>{{item.summary}}</p><ArrowRight :size="18" aria-hidden="true" /></NuxtLink></div><div v-else class="empty-notice"><BookOpen :size="24" aria-hidden="true"/><p>暂时没有已发布的路线。</p></div>
      </div>

      <!-- 短页靠底，长页随正文排列 -->
      <footer class="footer-tagline">
        LEARN WITH INTENTION. BUILD WITH CONFIDENCE.
      </footer>
    </div>
  </div>
</template>

<style scoped>
.home-placeholder{height:188px;background:var(--soft);border-radius:11px;margin-bottom:18px}.empty-doodle{color:var(--learning-spark);transform:rotate(-8deg);margin:0 auto 12px}.card:hover .art-index-pill{transform:rotate(-8deg) scale(1.08);border-color:var(--brand);color:var(--brand)}.art-index-pill{transition:transform .22s cubic-bezier(0.34,1.56,0.64,1),border-color .18s ease,color .18s ease}@media(prefers-reduced-motion:reduce){.art-index-pill{transition:none}.card:hover .art-index-pill{transform:none}}

.module-cover{width:100%;height:100%;object-fit:cover;position:absolute;inset:0}.discovery-controls{display:flex;align-items:center;gap:20px;margin:-4px 0 18px;color:var(--muted);font-size:12px}.discovery-controls>span{margin-right:auto}.discovery-controls label{display:flex;align-items:center;gap:8px}.discovery-controls select{background:var(--paper);color:var(--ink);border:1px solid var(--line);padding:6px 9px;border-radius:6px}.resume-action{font-size:12px;white-space:nowrap}.recent-learning { position: relative; overflow: hidden; display: flex; align-items: center; justify-content: space-between; gap: 24px; padding: 18px 24px; margin-bottom: 22px; border: 1px solid var(--border-playful); border-radius: 14px; background: linear-gradient(105deg, rgba(255, 175, 136, 0.16) 0%, rgba(239, 124, 141, 0.08) 45%, var(--paper) 90%); box-shadow: var(--surface-highlight), var(--card-shadow); color: var(--ink); text-decoration: none; transition: transform 0.22s cubic-bezier(0.34, 1.56, 0.64, 1), border-color 0.2s ease; }
.recent-learning::before { content: ''; position: absolute; left: 0; top: 0; bottom: 0; width: 4px; background: var(--gradient-collision); }
.recent-learning:hover { transform: translateY(-2px); border-color: var(--brand); }
.resume-action { padding: 7px 14px; border-radius: 6px; background: var(--btn-primary-bg); color: var(--btn-primary-fg); font-weight: 600; box-shadow: var(--btn-primary-shadow); }
.recent-copy { flex: 1; min-width: 0; display:grid;grid-template-columns:minmax(0,1fr) 140px;column-gap:20px; }
.recent-label { grid-column:1; font-size: 11px; color: var(--muted); }
.recent-learning h2 { grid-column:1; font-size: 15px; margin: 3px 0; font-weight: 650; }
.recent-learning p { grid-column:1; font-size: 12px; color: var(--muted); margin: 0 0 6px; }
.recent-progress { grid-column:2;grid-row:2;align-self:center; height: 4px; width: min(100%, 320px); border-radius: 4px; background: var(--line); overflow: hidden; }
.recent-progress span { display: block; height: 100%; background: var(--gradient-collision); border-radius: inherit; }
.recent-count { grid-column:2;grid-row:3; display: block; margin-top: 0; font-size: 11px; color: var(--muted); }

.learn-page {
  flex:1;
  display:flex;
  flex-direction:column;
  width: 100%;
  min-height: calc(100dvh - 68px);
  background: transparent;
  color: var(--ink);
}

.learn-container{flex:1;display:flex;flex-direction:column}

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
  color: var(--brand);
  border-bottom-color: var(--brand);
  font-weight: 650;
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
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  border-color: transparent;
  box-shadow: var(--btn-primary-shadow);
  font-weight: 600;
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
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
}

.card {
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.03) 0%, rgba(255, 255, 255, 0) 100%), var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
  box-shadow: var(--surface-highlight);
  overflow: hidden;
  text-decoration: none;
  color: var(--ink);
  display: flex;
  flex-direction: column;
  animation: surface-rise 0.32s cubic-bezier(0.22, 1, 0.36, 1) both;
  transition: transform 0.22s cubic-bezier(0.22, 1, 0.36, 1), box-shadow 0.22s ease, border-color 0.22s ease;
}

.card:hover {
  transform: translateY(-5px);
  border-color: var(--border-playful);
  box-shadow: var(--surface-highlight), 0 16px 38px -8px rgba(239, 124, 141, 0.22), 0 0 0 1px rgba(255, 175, 136, 0.2);
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
  display: inline-flex;
  align-items: center;
  border: 1px solid transparent;
  border-radius: 6px;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 550;
  color: var(--muted);
  background: var(--soft);
}

.tag.beginner {
  color: var(--tone-green);
  background: var(--tone-green-soft);
  border-color: color-mix(in srgb, var(--tone-green) 22%, transparent);
}

.tag.intermediate {
  color: var(--tone-blue);
  background: var(--tone-blue-soft);
  border-color: color-mix(in srgb, var(--tone-blue) 22%, transparent);
}

.tag.advanced {
  color: var(--tone-red);
  background: var(--tone-red-soft);
  border-color: color-mix(in srgb, var(--tone-red) 22%, transparent);
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
.published-routes {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
}

.published-route {
  position: relative;
  display: block;
  padding: 28px;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 10px;
  color: var(--ink);
  text-decoration: none;
  transition: border-color 0.15s ease, transform 0.15s ease;
}

.published-route:hover {
  border-color: var(--focus);
  transform: translateY(-2px);
}

.published-route span {
  color: var(--green);
  font-size: 12px;
}

.published-route > svg {
  position: absolute;
  right: 28px;
  top: 28px;
}

.published-route h2 {
  overflow-wrap: anywhere;
  font-size: 22px;
  margin: 14px 0;
}

.published-route p {
  font-size: 13px;
  line-height: 1.7;
  color: var(--muted);
  margin: 0;
}

/* 底部标语 */
.footer-tagline {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 10px;
  letter-spacing: 1.5px;
  color: var(--muted);
  text-align: center;
  margin-top: auto;
  padding-top: 48px;
  opacity: 0.7;
}

/* 响应式调整 */
@media (max-width: 900px) {
  .cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .discovery-controls{flex-wrap:wrap;gap:10px}.recent-copy{display:block}.recent-progress{margin-top:8px}
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
