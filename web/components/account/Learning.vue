<script setup lang="ts">
import { BookOpen, Play } from 'lucide-vue-next'
import type { LearningHistoryItem } from '~/shared/learning'

const { currentUser } = useAuth()
const request = useRequestFetch()
const filter = ref('all')
const filters = [
  { id: 'all', label: '全部' },
  { id: 'active', label: '进行中' },
  { id: 'completed', label: '已完成' },
]
const { data, pending, error, refresh } = await useLazyAsyncData(
  () => `account:learning:${currentUser.value?.id || 'guest'}`,
  async () => {
    const owner = currentUser.value?.id
    if (!owner) return []
    const result = await request<{ data: LearningHistoryItem[] }>(
      '/api/me/learning',
    )
    return currentUser.value?.id === owner ? result.data : []
  },
)
const records = computed(() => data.value || [])
const resume = computed(() =>
  records.value.find(
    (item) => !item.progress.isCompleted && item.progress.resumeChapterSlug,
  ),
)
const visible = computed(() =>
  records.value.filter(
    (item) =>
      filter.value === 'all' ||
      (filter.value === 'completed'
        ? item.progress.isCompleted
        : !item.progress.isCompleted),
  ),
)
function destination(item: LearningHistoryItem) {
  const base = `/learn/modules/${encodeURIComponent(item.slug)}`
  return !item.progress.isCompleted && item.progress.resumeChapterSlug
    ? `${base}/${encodeURIComponent(item.progress.resumeChapterSlug)}`
    : base
}
</script>

<template>
  <section aria-labelledby="personal-learning-title">
    <header class="personal-section-heading">
      <div>
        <span class="personal-eyebrow">LEARNING</span>
        <h1 id="personal-learning-title">学习记录</h1>
      </div>
      <NuxtLink to="/learn" class="personal-text-link">浏览模块</NuxtLink>
    </header>
    <div v-if="pending && !data" class="personal-loading" role="status">
      <UiBusyIndicator inline class="personal-loading-mark" aria-hidden="true"/><span class="personal-skeleton personal-skeleton-title" />
      <span class="personal-skeleton" />
      <span class="personal-skeleton" />
      <span class="personal-sr-only">正在读取学习记录…</span>
    </div>
    <div v-else-if="error" class="personal-empty" role="alert">
      <BookOpen :size="27" aria-hidden="true" />
      <h2>暂时无法读取学习记录</h2>
      <p>你的进度不会受影响，请稍后重试。</p>
      <button
        type="button"
        class="personal-button secondary"
        @click="refresh()"
      >
        重试
      </button>
    </div>
    <template v-else-if="records.length">
      <section
        v-if="resume"
        class="personal-resume"
        aria-labelledby="personal-resume-title"
      >
        <div class="personal-resume-copy">
          <span class="personal-resume-label">
            <span />
            继续上次的学习
          </span>
          <h2 id="personal-resume-title">{{ resume.title }}</h2>
          <p>{{ resume.chapterTitle || '继续未完成的章节' }}</p>
          <NuxtLink :to="destination(resume)" class="personal-resume-button">
            <Play :size="14" aria-hidden="true" />
            继续学习
          </NuxtLink>
        </div>
        <div class="personal-resume-progress">
          <strong>
            {{ resume.progress.percent }}
            <span>%</span>
          </strong>
          <span>模块进度</span>
          <span class="personal-progress">
            <i :style="{ width: `${resume.progress.percent}%` }" />
          </span>
          <small>
            {{ resume.progress.completedChapters }} /
            {{ resume.progress.totalChapters }} 章节已完成
          </small>
        </div>
      </section>
      <div class="personal-list-heading">
        <h2>最近学习</h2>
        <div class="personal-filters" role="group" aria-label="筛选学习记录">
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
          :key="item.moduleId"
          :title="item.title"
          :to="destination(item)"
          :subtitle="
            item.progress.isCompleted
              ? '所有章节已完成，可重新阅读'
              : item.chapterTitle || '查看模块内容'
          "
          :completed="item.progress.completedChapters"
          :total="item.progress.totalChapters"
          :complete="item.progress.isCompleted"
          :updated-at="item.lastVisitedAt"
          kind="learning"
        />
      </div>
      <div v-else class="personal-filter-empty">
        <p>
          {{
            filter === 'completed'
              ? '还没有已完成的模块。'
              : '当前记录中的模块都已完成。'
          }}
        </p>
        <button
          type="button"
          class="personal-text-link"
          @click="filter = 'all'"
        >
          查看全部记录
        </button>
      </div>
      <p v-if="records.length === 20" class="personal-footnote">
        显示最近学习的 20 个已发布模块。
      </p>
    </template>
    <div v-else class="personal-empty">
      <span class="personal-empty-icon">
        <BookOpen :size="28" aria-hidden="true" />
      </span>
      <h2>从一个感兴趣的模块开始</h2>
      <p>阅读章节后，进度会自动保存在这里。</p>
      <NuxtLink to="/learn" class="personal-button">浏览学习模块</NuxtLink>
    </div>
  </section>
</template>
