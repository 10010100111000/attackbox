<script setup lang="ts">
import type { NotificationItem, NotificationList } from '~/shared/notifications'
import { Bell, Trash2, Mail, MessageCircle, ArrowLeft } from 'lucide-vue-next'
import { apiErrorMessage, apiFetch } from '~/utils/api'

definePageMeta({ layout: 'learning' })
useHead({ title: '通知 — AttackBox' })

const { currentUser } = useAuth()
const { setUnread } = useNotifications()
const request = useRequestFetch()
const container = ref<HTMLElement | null>(null)

function positionMobile() {
  if (import.meta.client && window.matchMedia('(max-width:760px)').matches) {
    void nextTick(() => container.value?.scrollIntoView({ block: 'start' }))
  }
}

function back() {
  selected.value = null
  positionMobile()
}

const page = ref(1)
const selected = ref<NotificationItem | null>(null)
const message = ref('')
const deleting = ref(false)
let mutations = Promise.resolve()
let alive = true

onBeforeUnmount(() => {
  alive = false
})

const { data, pending, error, refresh } = await useLazyAsyncData(
  () => `account:notifications:${currentUser.value?.id || 'guest'}:${page.value}`,
  async () => {
    if (!currentUser.value) return null
    const owner = currentUser.value.id
    const r = await request<{ data: NotificationList }>('/api/me/notifications', {
      query: { page: page.value }
    })
    if (currentUser.value?.id !== owner) return null
    setUnread(r.data.unread)
    return r.data
  }
)

watch([page, () => currentUser.value?.id], () => {
  selected.value = null
  message.value = ''
})

function enqueue(task: () => Promise<void>) {
  mutations = mutations.catch(() => {}).then(task)
  return mutations
}

function read(item: NotificationItem) {
  selected.value = item
  message.value = ''
  positionMobile()
  if (item.readAt) return
  const owner = currentUser.value?.id
  void enqueue(async () => {
    if (!alive || currentUser.value?.id !== owner || !data.value?.items.some(n => n.id === item.id))
      return
    try {
      const r = await apiFetch<{ unread: number }>(`/api/me/notifications/${item.id}/read`, {
        method: 'POST',
        retry: false
      })
      if (alive && currentUser.value?.id === owner) {
        const readAt = new Date().toISOString()
        if (data.value) {
          data.value = {
            ...data.value,
            unread: r.unread,
            items: data.value.items.map(n => (n.id === item.id ? { ...n, readAt } : n))
          }
        }
        if (selected.value?.id === item.id) {
          selected.value = { ...selected.value, readAt }
        }
        setUnread(r.unread)
      }
    } catch (e) {
      if (alive && currentUser.value?.id === owner && selected.value?.id === item.id) {
        message.value = apiErrorMessage(e, '已读状态保存失败')
      }
    }
  })
}

async function remove() {
  if (!selected.value || deleting.value) return
  const owner = currentUser.value?.id
  const id = selected.value.id
  deleting.value = true
  message.value = ''
  try {
    await enqueue(async () => {
      if (!alive || currentUser.value?.id !== owner) return
      const r = await apiFetch<{ deleted: true; unread: number }>(`/api/me/notifications/${id}`, {
        method: 'DELETE',
        retry: false
      })
      if (!alive || currentUser.value?.id !== owner || !data.value) return
      data.value = {
        ...data.value,
        items: data.value.items.filter(item => item.id !== id),
        total: Math.max(0, data.value.total - 1),
        unread: r.unread
      }
      setUnread(r.unread)
      if (selected.value?.id === id) selected.value = null
      if (!data.value.items.length && page.value > 1) page.value--
    })
  } catch (e) {
    if (alive && currentUser.value?.id === owner) {
      message.value = apiErrorMessage(e, '删除失败，请重试')
    }
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <AccountShell section="notifications">
    <section ref="container" class="notification-section" aria-labelledby="notification-title">
      <header class="personal-section-heading">
        <h1 id="notification-title">通知</h1>
        <span v-if="data?.unread" class="notification-count"> {{ data.unread }} 条未读 </span>
      </header>

      <p v-if="error" role="alert">
        消息读取失败。
        <button class="personal-text-link" @click="refresh()">重试</button>
      </p>

      <UiBusyIndicator v-else-if="pending && !data" label="通知加载" />

      <div
        v-else-if="data?.items.length"
        class="notification-grid"
        :class="{ 'detail-open': !!selected }"
      >
        <section class="notification-list" aria-label="通知列表">
          <button
            v-for="item in data.items"
            :key="item.id"
            :class="{ selected: selected?.id === item.id, unread: !item.readAt }"
            :aria-pressed="selected?.id === item.id"
            @click="read(item)"
          >
            <span class="notification-kind" :class="item.kind">
              <Bell v-if="item.kind === 'announcement'" :size="12" />
              <MessageCircle v-else :size="12" />
              {{ item.kind === 'announcement' ? '平台通知' : '消息' }}
            </span>
            <strong>{{ item.title }}</strong>
            <small>
              <span>{{ item.sender.name }}</span>
              <time>{{ item.publishedAt.slice(0, 10) }}</time>
            </small>
            <i v-if="!item.readAt" class="unread-dot" aria-label="未读" />
          </button>

          <nav v-if="data.total > data.pageSize" class="notification-pagination">
            <button :disabled="page === 1 || pending" @click="page--">上一页</button>
            <span>{{ page }}</span>
            <button :disabled="page * data.pageSize >= data.total || pending" @click="page++">
              下一页
            </button>
          </nav>
        </section>

        <article v-if="selected" class="notification-detail">
          <button type="button" class="notification-back" @click="back">
            <ArrowLeft :size="15" />
            全部消息
          </button>
          <div class="notification-detail-meta">
            <span class="notification-kind" :class="selected.kind">
              {{ selected.kind === 'announcement' ? '平台通知' : '消息' }}
            </span>
            <div class="notification-detail-actions">
              <time>{{ selected.publishedAt.slice(0, 10) }}</time>
              <button
                class="notification-delete"
                :disabled="deleting"
                aria-label="删除消息"
                @click="remove"
              >
                <UiBusyLabel :busy="deleting">
                  <Trash2 :size="15" />
                  删除
                </UiBusyLabel>
              </button>
            </div>
          </div>
          <h2>{{ selected.title }}</h2>
          <div class="notification-sender">
            <span class="sender-initial" aria-hidden="true">
              {{ selected.sender.name.slice(0, 1) }}
            </span>
            <span class="sender-name">{{ selected.sender.name }}</span>
          </div>
          <p class="notification-body">{{ selected.body }}</p>
          <NuxtLink v-if="selected.link" :to="selected.link" class="personal-text-link">
            查看相关内容
          </NuxtLink>
          <p v-if="message" role="alert" class="notification-error">
            {{ message }}
          </p>
        </article>

        <div v-else class="notification-pick">
          <Mail :size="28" />
          <span>选择消息查看详情</span>
        </div>
      </div>

      <div v-else class="personal-empty">
        <div class="personal-empty-icon">
          <Bell :size="28" />
        </div>
        <h2>暂无通知</h2>
      </div>
    </section>
  </AccountShell>
</template>

<style scoped>
.notification-section {
  scroll-margin-top: 84px;
}

.notification-back {
  display: none;
}

.notification-detail-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.notification-error {
  font-size: 12px;
  color: var(--error-ink);
}

.sender-name {
  min-width: 0;
  overflow-wrap: anywhere;
}

.notification-count {
  font-size: 12px;
  font-weight: 600;
  color: var(--brand);
  background: var(--brand-soft);
  padding: 5px 12px;
  border-radius: 20px;
}

.notification-grid {
  display: grid;
  grid-template-columns: 290px minmax(0, 1fr);
  min-height: 440px;
  max-height: min(680px, calc(100dvh - 220px));
  border: 1px solid var(--line);
  border-radius: 14px;
  background: var(--paper);
  box-shadow: var(--surface-highlight), var(--card-shadow);
  overflow: hidden;
}

.notification-list {
  min-height: 0;
  overflow: auto;
  border-right: 1px solid var(--line);
}

.notification-list > button {
  position: relative;
  display: grid;
  gap: 10px;
  width: 100%;
  text-align: left;
  padding: 20px 22px;
  border: 0;
  border-bottom: 1px solid var(--line);
  background: transparent;
  color: var(--ink);
  cursor: pointer;
  transition: background 0.15s ease;
}

.notification-list > button.selected {
  background: var(--soft);
  box-shadow: inset 3px 0 var(--brand);
}

.notification-list > button:hover:not(.selected) {
  background: var(--hover);
}

.notification-list strong {
  font-size: 14px;
  font-weight: 500;
  overflow-wrap: anywhere;
}

.notification-list .unread strong {
  font-weight: 650;
}

.notification-list small {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 11px;
  color: var(--muted);
}

.notification-list small span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.unread-dot {
  position: absolute;
  right: 18px;
  top: 24px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--brand);
  box-shadow: 0 0 8px var(--brand);
}

.notification-kind {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  width: fit-content;
  font-size: 10px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 6px;
  color: var(--tone-blue);
  background: var(--tone-blue-soft);
}

.notification-kind.message {
  color: var(--tone-green);
  background: var(--tone-green-soft);
}

.notification-detail {
  min-height: 0;
  overflow: auto;
  padding: 28px 34px;
  overflow-wrap: anywhere;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.notification-detail-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.notification-detail time {
  color: var(--muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.notification-detail h2 {
  font-size: 22px;
  font-weight: 650;
  line-height: 1.4;
  margin: 20px 0 14px;
  letter-spacing: -0.4px;
}

.notification-sender {
  display: flex;
  align-items: center;
  gap: 9px;
  font-size: 12px;
  color: var(--muted);
  padding-bottom: 18px;
  border-bottom: 1px solid var(--line);
}

.sender-initial {
  flex-shrink: 0;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 9px;
  background: var(--avatar-bg);
  color: var(--avatar-fg);
  border: 1px solid var(--avatar-border);
  font-size: 12px;
  font-weight: 650;
}

.notification-body {
  font-size: 14px;
  line-height: 1.9;
  white-space: pre-wrap;
  margin: 22px 0;
  color: var(--body);
}

.notification-delete {
  border: 1px solid var(--line);
  background: transparent;
  color: var(--error-ink);
  padding: 7px 11px;
  border-radius: 7px;
  font-size: 12px;
  cursor: pointer;
  transition: background 0.15s ease;
}

.notification-delete:hover:not(:disabled) {
  background: var(--error-bg);
}

.notification-delete:disabled {
  opacity: 0.5;
  cursor: default;
}

.notification-pick {
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 12px;
  color: var(--muted);
  font-size: 12px;
}

.notification-pick svg {
  color: var(--muted);
  opacity: 0.65;
}

.notification-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  font-size: 12px;
}

.notification-pagination button {
  background: transparent;
  color: var(--ink);
  border: 0;
  cursor: pointer;
}

.notification-pagination button:disabled {
  opacity: 0.4;
}

button:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: -3px;
}

@media (max-width: 1100px) {
  .notification-grid {
    grid-template-columns: 235px minmax(0, 1fr);
  }
}

@media (max-width: 760px) {
  .notification-grid {
    grid-template-columns: 1fr;
    max-height: none;
  }
  .notification-list {
    border-right: 0;
    overflow: visible;
  }
  .notification-grid.detail-open .notification-list {
    display: none;
  }
  .notification-detail {
    padding: 24px;
    min-height: 420px;
    overflow: visible;
  }
  .notification-back {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    width: fit-content;
    margin-bottom: 18px;
    min-height: 36px;
    border: 0;
    background: transparent;
    color: var(--ink);
    font-weight: 550;
    padding: 0;
    font-size: 12px;
    cursor: pointer;
  }
  .notification-delete {
    min-height: 44px;
  }
  .notification-pick {
    display: none;
  }
}
</style>
