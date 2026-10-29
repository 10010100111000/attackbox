<script setup lang="ts">
import { Flag } from 'lucide-vue-next'
import type { ChallengeSummary } from '~/shared/challenges'

const { currentUser } = useAuth()
const request = useRequestFetch()
const filter = ref('all')
const filters = [
  { id: 'all', label: '全部' },
  { id: 'active', label: '进行中' },
  { id: 'completed', label: '已完成' }
]
const difficulties: Record<string, string> = {
  beginner: '入门',
  intermediate: '进阶',
  advanced: '高级'
}
const { data, pending, error, refresh } = await useLazyAsyncData(
  () => `account:challenges:${currentUser.value?.id || 'guest'}`,
  async () => {
    const owner = currentUser.value?.id
    if (!owner) return []
    const result = await request<{ data: ChallengeSummary[] }>('/api/me/challenges')
    return currentUser.value?.id === owner ? result.data : []
  }
)
const records = computed(() => data.value || [])
function complete(item: ChallengeSummary) {
  return item.totalStages > 0 && item.completedStages === item.totalStages
}
const visible = computed(() =>
  records.value.filter(
    item =>
      filter.value === 'all' || (filter.value === 'completed' ? complete(item) : !complete(item))
  )
)
</script>

<template>
  <section aria-labelledby="personal-challenges-title">
    <header class="personal-section-heading">
      <div>
        <h1 id="personal-challenges-title">挑战记录</h1>
      </div>
      <NuxtLink to="/challenges" class="personal-text-link">探索挑战</NuxtLink>
    </header>
    <div v-if="pending && !data" class="personal-loading" role="status">
      <UiBusyIndicator inline class="personal-loading-mark" aria-hidden="true" /><span
        class="personal-skeleton personal-skeleton-title"
      />
      <span class="personal-skeleton" />
      <span class="personal-skeleton" />
      <span class="personal-sr-only">正在读取挑战记录…</span>
    </div>
    <div v-else-if="error" class="personal-empty" role="alert">
      <Flag :size="27" aria-hidden="true" />
      <h2>暂时无法读取挑战记录</h2>
      <p>已通过的阶段不会丢失，请稍后重试。</p>
      <button type="button" class="personal-button secondary" @click="refresh()">重试</button>
    </div>
    <template v-else-if="records.length">
      <div class="personal-list-heading first">
        <h2>已通过阶段的挑战</h2>
        <div class="personal-filters" role="group" aria-label="筛选挑战记录">
          <button
            v-for="item in filters"
            :key="item.id"
            type="button"
            :aria-pressed="filter === item.id"
            @click="filter = item.id"
          >
            {{ item.label }}
          </button>
        </div>
      </div>
      <div v-if="visible.length" class="personal-record-list">
        <AccountRecord
          v-for="item in visible"
          :key="item.id"
          :title="item.title"
          :to="`/challenges/${item.id}`"
          :subtitle="[item.category, difficulties[item.difficulty]].filter(Boolean).join(' · ')"
          :completed="item.completedStages"
          :total="item.totalStages"
          :complete="complete(item)"
          :updated-at="item.lastPassedAt"
          kind="challenge"
        />
      </div>
      <div v-else class="personal-filter-empty">
        <p>
          {{ filter === 'completed' ? '还没有全部通关的挑战。' : '当前记录中的挑战都已通关。' }}
        </p>
        <button type="button" class="personal-text-link" @click="filter = 'all'">
          查看全部记录
        </button>
      </div>
      <p class="personal-footnote">记录至少通过一个阶段的已发布挑战，按最近通过时间排序。</p>
    </template>
    <div v-else class="personal-empty">
      <span class="personal-empty-icon">
        <Flag :size="28" aria-hidden="true" />
      </span>
      <h2>拿下你的第一个 Flag</h2>
      <p>通过挑战的任意阶段后，记录会出现在这里。</p>
      <NuxtLink to="/challenges" class="personal-button">探索挑战</NuxtLink>
    </div>
  </section>
</template>
