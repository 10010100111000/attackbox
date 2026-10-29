<script setup lang="ts">
import { Inbox, MessageSquarePlus, ExternalLink } from 'lucide-vue-next'
import {
  feedbackCategories,
  feedbackStatuses,
  type FeedbackList,
  type FeedbackStatus
} from '~/shared/feedback'
import { apiErrorMessage, apiFetch } from '~/utils/api'

definePageMeta({ layout: 'admin' })
useHead({ title: '用户反馈 — AttackBox' })
const query = ref<{ page: number; status: 'all' | FeedbackStatus }>({ page: 1, status: 'all' })
const selectedId = ref(''),
  saving = ref(false),
  message = ref(''),
  saveError = ref('')
const { data, pending, error, refresh } = await useLazyFetch<{ data: FeedbackList }>(
  '/api/admin/feedback',
  {
    key: 'admin:feedback',
    query,
    timeout: 15000
  }
)
const result = computed(() => data.value?.data)
const selected = computed(
  () => result.value?.items.find(item => item.id === selectedId.value) || result.value?.items[0]
)
const total = computed(() => {
  const counts = result.value?.counts
  return counts ? counts.new + counts.in_progress + counts.resolved : 0
})
const filters = computed(() => [
  { value: 'all' as const, label: '全部', count: total.value },
  ...feedbackStatuses.map(option => ({ ...option, count: result.value?.counts[option.value] || 0 }))
])
const categoryLabel = (category: string) =>
  feedbackCategories.find(option => option.value === category)?.label || category
const statusLabel = (status: string) =>
  feedbackStatuses.find(option => option.value === status)?.label || status
// Fixed formatting avoids server/browser locale or timezone hydration differences.
const dateLabel = (date: string) => date.slice(0, 10)
function filter(value: 'all' | FeedbackStatus) {
  if (saving.value) return
  query.value = { page: 1, status: value }
  selectedId.value = ''
  message.value = ''
  saveError.value = ''
}
function select(id: string) {
  selectedId.value = id
  saveError.value = ''
  message.value = ''
}
async function changeStatus(status: FeedbackStatus) {
  if (!selected.value || saving.value || status === selected.value.status) return
  const id = selected.value.id
  saving.value = true
  message.value = ''
  saveError.value = ''
  try {
    await apiFetch(`/api/admin/feedback/${id}`, {
      method: 'PATCH',
      body: { status },
      retry: false,
      timeout: 15000
    })
    await refresh()
    if (result.value && !result.value.items.length && query.value.page > 1) {
      query.value = { ...query.value, page: query.value.page - 1 }
    }
    message.value = `已标记为${statusLabel(status)}`
  } catch (e) {
    saveError.value = apiErrorMessage(e, '保存未确认，请重新查看反馈状态。')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <section class="feedback-admin">
    <header class="feedback-heading">
      <div>
        <h1>用户反馈</h1>
        <p>从用户遇到的问题，找到下一步改进。</p>
      </div>
      <span v-if="result" class="feedback-total">{{ total }} 条反馈</span>
    </header>
    <nav class="feedback-filters" aria-label="反馈筛选">
      <button
        v-for="item in filters"
        :key="item.value"
        :aria-pressed="query.status === item.value"
        :class="{ active: query.status === item.value }"
        :disabled="saving"
        @click="filter(item.value)"
      >
        {{ item.label }}<span v-if="result">{{ item.count }}</span>
      </button>
    </nav>
    <UiNotice v-if="error" variant="error" class="mb-4">
      <div class="flex items-center justify-between gap-3 w-full">
        <span>反馈列表加载失败</span>
        <UiButton variant="secondary" size="xs" type="button" @click="refresh()">重新加载</UiButton>
      </div>
    </UiNotice>
    <UiBusyIndicator v-else-if="pending && !result" label="反馈加载" />
    <template v-else>
      <UiNotice v-if="message" variant="success" class="mb-4">{{ message }}</UiNotice>
      <UiNotice v-if="saveError" variant="error" class="mb-4">{{ saveError }}</UiNotice>
      <UiEmptyState
        v-if="!result?.items.length"
        :title="query.status === 'all' ? '还没有反馈' : '这个状态下还没有反馈'"
        description="用户可以从头像菜单提交问题、纠错或建议。"
      >
        <template #icon>
          <Inbox :size="32" />
        </template>
      </UiEmptyState>
      <div v-else class="feedback-workspace" :aria-busy="pending">
        <section class="feedback-inbox" aria-label="反馈列表">
          <button
            v-for="item in result.items"
            :key="item.id"
            class="feedback-row"
            :class="{ selected: selected?.id === item.id }"
            :aria-pressed="selected?.id === item.id"
            :disabled="saving"
            @click="select(item.id)"
          >
            <div class="row-top">
              <span class="feedback-category">{{ categoryLabel(item.category) }}</span>
              <UiBadge
                :variant="
                  item.status === 'resolved'
                    ? 'success'
                    : item.status === 'in_progress'
                      ? 'brand'
                      : 'warning'
                "
                dot
              >
                {{ statusLabel(item.status) }}
              </UiBadge>
            </div>
            <p>{{ item.body }}</p>
            <div class="row-meta">
              <span>{{ item.author.name }}</span
              ><time :datetime="item.createdAt">{{ dateLabel(item.createdAt) }}</time>
            </div>
          </button>
          <nav
            v-if="result.total > result.pageSize"
            class="feedback-pagination"
            aria-label="反馈分页"
          >
            <UiButton
              variant="secondary"
              size="xs"
              :disabled="query.page === 1 || saving || pending"
              @click="query = { ...query, page: query.page - 1 }"
              >上一页</UiButton
            >
            <span class="text-xs text-muted"
              >{{ query.page }} / {{ Math.ceil(result.total / result.pageSize) }}</span
            >
            <UiButton
              variant="secondary"
              size="xs"
              :disabled="query.page * result.pageSize >= result.total || saving || pending"
              @click="query = { ...query, page: query.page + 1 }"
              >下一页</UiButton
            >
          </nav>
        </section>
        <article v-if="selected" class="feedback-detail" aria-label="反馈详情">
          <header>
            <div class="detail-icon" aria-hidden="true"><MessageSquarePlus :size="20" /></div>
            <div>
              <h2>{{ categoryLabel(selected.category) }}</h2>
              <span>{{ dateLabel(selected.createdAt) }}</span>
            </div>
            <UiBadge
              :variant="
                selected.status === 'resolved'
                  ? 'success'
                  : selected.status === 'in_progress'
                    ? 'brand'
                    : 'warning'
              "
              dot
            >
              {{ statusLabel(selected.status) }}
            </UiBadge>
          </header>
          <div class="feedback-author">
            <strong>{{ selected.author.name }}</strong
            ><span>{{ selected.author.email || '账户已注销' }}</span>
          </div>
          <p class="feedback-body">{{ selected.body }}</p>
          <a
            v-if="selected.sourcePath"
            :href="selected.sourcePath"
            target="_blank"
            rel="noopener noreferrer"
            class="feedback-source"
            ><span>相关页面</span><code>{{ selected.sourcePath }}</code
            ><ExternalLink :size="14"
          /></a>
          <footer>
            <span>处理状态</span>
            <div role="group" aria-label="处理状态">
              <button
                v-for="option in feedbackStatuses"
                :key="option.value"
                :aria-pressed="selected.status === option.value"
                :class="{ active: selected.status === option.value }"
                :disabled="saving || pending || selected.status === option.value"
                @click="changeStatus(option.value)"
              >
                {{ option.label }}
              </button>
            </div>
            <UiBusyIndicator v-if="saving" inline label="保存反馈状态" />
          </footer>
        </article>
      </div>
    </template>
  </section>
</template>

<style scoped>
.feedback-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 28px;
}
h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  letter-spacing: -0.5px;
}
.feedback-heading p {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--muted);
}
.feedback-total {
  font-size: 12px;
  color: var(--muted);
}
.feedback-filters {
  display: flex;
  gap: 4px;
  padding: 4px;
  width: fit-content;
  margin-bottom: 24px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--paper);
}
.feedback-filters button {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 8px 14px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  cursor: pointer;
}
.feedback-filters button.active {
  background: var(--soft);
  color: var(--ink);
}
.feedback-filters button > span {
  font-size: 10px;
  font-variant-numeric: tabular-nums;
}
.feedback-workspace {
  display: grid;
  grid-template-columns: minmax(320px, 0.85fr) minmax(400px, 1.15fr);
  gap: 24px;
  align-items: start;
}
.feedback-inbox {
  display: grid;
  gap: 10px;
  min-width: 0;
}
.feedback-row {
  display: block;
  width: 100%;
  text-align: left;
  padding: 18px;
  border: 1px solid var(--line);
  border-radius: 12px;
  background: var(--paper);
  color: var(--ink);
  cursor: pointer;
}
.feedback-row:hover {
  background: var(--soft);
}
.feedback-row.selected {
  border-color: var(--brand);
  box-shadow: inset 3px 0 0 var(--brand);
}
.row-top,
.row-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
.feedback-category {
  font-size: 12px;
  font-weight: 550;
}
.feedback-row p {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  margin: 14px 0;
  font-size: 13px;
  line-height: 1.8;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.row-meta {
  color: var(--muted);
  font-size: 11px;
}
.row-meta > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.row-meta time {
  flex-shrink: 0;
}
.feedback-detail {
  position: sticky;
  top: 24px;
  min-width: 0;
  padding: 26px;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: var(--paper);
}
.feedback-detail > header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 22px;
  border-bottom: 1px solid var(--line);
}
.detail-icon {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 11px;
  background: var(--brand-soft);
  color: var(--brand);
}
.feedback-detail h2 {
  margin: 0 0 5px;
  font-size: 16px;
  font-weight: 600;
}
.feedback-detail header > div > span {
  color: var(--muted);
  font-size: 11px;
}
.feedback-author {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  align-items: baseline;
  margin-top: 22px;
  font-size: 12px;
}
.feedback-author strong {
  font-weight: 550;
}
.feedback-author span {
  color: var(--muted);
  overflow-wrap: anywhere;
}
.feedback-body {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font-size: 14px;
  line-height: 1.9;
  margin: 20px 0 30px;
}
.feedback-source {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 12px;
  border-radius: 8px;
  background: var(--soft);
  font-size: 11px;
  color: var(--muted);
  text-decoration: none;
}
.feedback-source > span {
  flex-shrink: 0;
}
.feedback-source code {
  color: var(--ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.feedback-source svg {
  flex-shrink: 0;
  margin-left: auto;
}
.feedback-source:hover {
  color: var(--brand);
}
.feedback-detail footer {
  display: flex;
  align-items: center;
  gap: 14px;
  padding-top: 24px;
  margin-top: 24px;
  border-top: 1px solid var(--line);
}
.feedback-detail footer > span {
  font-size: 12px;
  color: var(--muted);
}
.feedback-detail footer > div {
  display: flex;
  gap: 6px;
}
.feedback-detail footer button {
  padding: 8px 12px;
  border: 1px solid var(--line);
  border-radius: 7px;
  background: transparent;
  color: var(--body);
  font-size: 11px;
  cursor: pointer;
}
.feedback-detail footer button.active {
  background: var(--brand-soft);
  border-color: var(--brand);
  color: var(--brand);
  opacity: 1;
}
.feedback-pagination {
  display: flex;
  gap: 14px;
  align-items: center;
  justify-content: space-between;
  margin-top: 8px;
  color: var(--muted);
  font-size: 11px;
}
.feedback-row:disabled,
.feedback-filters button:disabled,
.feedback-detail footer button:disabled {
  cursor: default;
  opacity: 0.55;
}
button:focus-visible,
a:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 3px;
}
</style>
