<script setup lang="ts">
import { ref, computed } from 'vue'
import { Flag, ArrowRight, CheckCircle2 } from 'lucide-vue-next'
import { useAuth } from '~/composables/useAuth'
import type { ChallengeSummary } from '~/shared/challenges'

definePageMeta({ layout: 'learning' })
useHead({ title: '挑战 — AttackBox' })

const { currentUser } = useAuth()
const learner = computed(() => currentUser.value?.id || '')

const { data, pending, error, refresh } = useCatalogData<{ data: ChallengeSummary[] }>(
  () => `challenges:list:${learner.value || 'guest'}`,
  '/api/challenges'
)

const query = ref('')
const category = ref('')
const difficulty = ref('')

const categories = computed(() => [
  ...new Set((data.value?.data || []).map(c => c.category).filter((c): c is string => Boolean(c)))
])

const rows = computed(
  () =>
    data.value?.data.filter(
      c =>
        (!category.value || c.category === category.value) &&
        (!difficulty.value || c.difficulty === difficulty.value) &&
        c.title.toLowerCase().includes(query.value.toLowerCase())
    ) || []
)

const labels: Record<string, string> = {
  beginner: '入门',
  intermediate: '进阶',
  advanced: '高级'
}

const categoryOptions = computed(() => [
  { value: '', label: '全部分类' },
  ...categories.value.map(category => ({ value: category, label: category }))
])
const difficultyOptions = computed(() => [
  { value: '', label: '全部难度' },
  ...Object.entries(labels).map(([value, label]) => ({ value, label }))
])

function stagePercent(c: ChallengeSummary): number {
  if (!currentUser.value || !c.totalStages) return 0
  return Math.min(100, Math.round((c.completedStages / c.totalStages) * 100))
}
</script>

<template>
  <div class="learning-shell challenge-list">
    <header class="page-head">
      <span class="head-badge" aria-hidden="true">
        <Flag :size="20" />
      </span>
      <h1>挑战</h1>
    </header>
    <p class="intro">从一个目标开始，逐步找到答案。</p>

    <div class="filters">
      <input v-model="query" aria-label="搜索挑战" placeholder="搜索挑战" />
      <UiSelect v-model="category" aria-label="挑战分类" :options="categoryOptions" />
      <UiSelect v-model="difficulty" aria-label="挑战难度" :options="difficultyOptions" />
    </div>

    <LearningCatalogSkeleton v-if="pending && !data" label="正在读取挑战" />
    <p v-else-if="error" role="alert">
      无法读取挑战。
      <button type="button" @click="refresh()">重试</button>
    </p>

    <div v-else class="cards">
      <NuxtLink
        v-for="c in rows"
        :key="c.id"
        :to="`/challenges/${c.id}`"
        class="card"
        :data-difficulty="c.difficulty"
      >
        <div class="meta">
          <div class="meta-tags">
            <span class="diff-pill" :class="c.difficulty">{{
              labels[c.difficulty] || '挑战'
            }}</span>
            <span class="cat-label">{{ c.category || '综合' }}</span>
          </div>
          <CheckCircle2
            v-if="currentUser && c.totalStages > 0 && c.completedStages === c.totalStages"
            :size="17"
            class="completed-icon"
            aria-hidden="true"
          />
        </div>
        <h2>{{ c.title }}</h2>
        <p>{{ c.summary }}</p>
        <footer>
          <div class="footer-row">
            <span class="stage-count">
              {{
                currentUser
                  ? `${c.completedStages} / ${c.totalStages} 阶段已完成`
                  : `${c.totalStages} 个阶段`
              }}
            </span>
            <ArrowRight :size="16" class="card-arrow" aria-hidden="true" />
          </div>
          <div v-if="currentUser && c.totalStages > 0" class="stage-bar" aria-hidden="true">
            <i :style="{ width: `${stagePercent(c)}%` }" />
          </div>
        </footer>
      </NuxtLink>
    </div>

    <div v-if="!pending && !error && !rows.length" class="empty">
      <Flag :size="27" class="empty-flag" aria-hidden="true" />
      <p>暂无符合条件的挑战。</p>
      <NuxtLink to="/learn">先挑个模块热热身</NuxtLink>
    </div>
  </div>
</template>

<style scoped>
.empty-flag {
  color: var(--learning-spark);
  transform: rotate(-8deg);
  margin: 0 auto 12px;
}
.empty a {
  color: var(--green);
  font-size: 13px;
}

.page-head {
  display: flex;
  gap: 12px;
  align-items: center;
}

.head-badge {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 10px;
  background: var(--soft);
  color: var(--brand);
  border: 1px solid var(--line);
  box-shadow: var(--surface-highlight);
}

h1 {
  font-size: 30px;
  margin: 0;
  letter-spacing: -0.6px;
}

.intro {
  color: var(--muted);
  font-size: 14px;
  margin: 14px 0 28px;
}

.filters {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 26px;
}

input,
select,
button {
  padding: 10px 12px;
  background: var(--paper);
  color: var(--ink);
  border: 1px solid var(--line);
  border-radius: 7px;
  font-size: 13px;
  box-sizing: border-box;
  transition: border-color 0.15s ease;
}

input:focus,
select:focus {
  outline: none;
  border-color: var(--focus);
}

input {
  max-width: 300px;
  flex: 1;
  min-width: 180px;
}

.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
}

.card {
  position: relative;
  background: var(--card-surface);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  border: 1px solid var(--line);
  border-radius: 12px;
  box-shadow: var(--surface-highlight), var(--card-shadow);
  padding: 24px 26px;
  text-decoration: none;
  color: var(--ink);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: surface-rise 0.28s cubic-bezier(0.22, 1, 0.36, 1) both;
  transition:
    border-color 0.22s ease,
    transform 0.22s cubic-bezier(0.22, 1, 0.36, 1),
    box-shadow 0.22s ease;
}

.card:hover {
  border-color: var(--brand-border);
  transform: translateY(-4px);
  box-shadow: var(--card-hover-shadow);
}

.meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--muted);
  font-size: 12px;
}

.meta-tags {
  display: flex;
  align-items: center;
  gap: 8px;
}

.diff-pill {
  font-size: 11px;
  font-weight: 550;
  padding: 2px 8px;
  border-radius: 5px;
  border: 1px solid transparent;
  background: var(--soft);
  color: var(--muted);
}

.diff-pill.beginner {
  color: var(--tone-green);
  background: var(--tone-green-soft);
}

.diff-pill.intermediate {
  color: var(--tone-blue);
  background: var(--tone-blue-soft);
}

.diff-pill.advanced {
  color: var(--tone-red);
  background: var(--tone-red-soft);
}

.cat-label {
  font-size: 12px;
  color: var(--muted);
}

.completed-icon {
  color: var(--success-ink);
}

h2 {
  font-size: 20px;
  font-weight: 650;
  letter-spacing: -0.3px;
  margin: 18px 0 10px;
}

.card p {
  font-size: 13px;
  color: var(--muted);
  line-height: 1.7;
  min-height: 44px;
  flex: 1;
  margin: 0;
}

footer {
  border-top: 1px solid var(--line);
  padding-top: 16px;
  margin-top: 22px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.footer-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.card-arrow {
  transition:
    transform 0.18s ease,
    color 0.18s ease;
}

.card:hover .card-arrow {
  transform: translateX(4px);
  color: var(--brand);
}

.stage-bar {
  height: 4px;
  border-radius: 4px;
  background: var(--soft);
  overflow: hidden;
}

.stage-bar i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--brand);
}

.empty {
  padding: 60px;
  text-align: center;
  color: var(--muted);
}
</style>
