<script setup lang="ts">
import type { NotificationItem, NotificationList } from '~/shared/notifications'
import { Bell, Check } from 'lucide-vue-next'
definePageMeta({ layout: 'learning' })
useHead({ title: '通知 — AttackBox' })
const { currentUser } = useAuth(),
  { setUnread } = useNotifications(),
  request = useRequestFetch(),
  page = ref(1),
  selected = ref<NotificationItem | null>(null),
  message = ref('')
const { data, pending, error } = await useLazyAsyncData(
  () =>
    `account:notifications:${currentUser.value?.id || 'guest'}:${page.value}`,
  async () => {
    if (!currentUser.value) return null
    const owner = currentUser.value.id
    const r = await request<{ data: NotificationList }>(
      '/api/me/notifications',
      { query: { page: page.value } },
    )
    return currentUser.value?.id === owner ? r.data : null
  },
)
watch([page, () => currentUser.value?.id], () => {
  selected.value = null
  message.value = ''
})
async function read(item: NotificationItem) {
  selected.value = item
  message.value = ''
  const owner = currentUser.value?.id
  try {
    const r = await $fetch<{ unread: number }>(
      `/api/me/notifications/${item.id}/read`,
      { method: 'POST', retry: false },
    )
    if (currentUser.value?.id === owner) {
      item.readAt = new Date().toISOString()
      setUnread(r.unread)
    }
  } catch (e: any) {
    message.value = e?.data?.message || '已读状态保存失败'
  }
}
</script>
<template>
  <div class="learning-shell notification-page">
    <header>
      <div>
        <h1>通知</h1>
        <p>课程更新与平台消息</p>
      </div>
      <Bell :size="23" />
    </header>
    <p v-if="!currentUser">
      登录后查看通知。
      <NuxtLink to="/auth">登录</NuxtLink>
    </p>
    <p v-else-if="error" role="alert">通知读取失败，请稍后重新进入。</p>
    <p v-else-if="pending && !data" role="status">正在读取通知…</p>
    <div v-else-if="data?.items.length" class="notification-grid">
      <section class="notification-list" aria-label="通知列表">
        <button
          v-for="item in data.items"
          :key="item.id"
          :class="{ selected: selected?.id === item.id, unread: !item.readAt }"
          @click="read(item)"
        >
          <span class="item-heading">
            <i v-if="!item.readAt" />
            <strong>{{ item.title }}</strong>
            <Check v-if="item.readAt" :size="13" />
          </span>
          <small>{{ item.publishedAt.slice(0, 10) }}</small>
        </button>
        <nav v-if="data.total > data.pageSize" class="pagination">
          <button :disabled="page === 1" @click="page--">上一页</button>
          <span>{{ page }}</span>
          <button
            :disabled="page * data.pageSize >= data.total"
            @click="page++"
          >
            下一页
          </button>
        </nav>
      </section>
      <article v-if="selected">
        <time>{{ selected.publishedAt.slice(0, 10) }}</time>
        <h2>{{ selected.title }}</h2>
        <p class="notification-body">{{ selected.body }}</p>
        <NuxtLink v-if="selected.link" :to="selected.link" class="detail-link">
          查看相关内容
        </NuxtLink>
        <p v-if="message" role="alert">{{ message }}</p>
      </article>
      <div v-else class="empty-detail">选择一条通知阅读</div>
    </div>
    <div v-else class="empty-state">
      <Bell :size="26" />
      <h2>暂无通知</h2>
      <p>新的平台消息会显示在这里。</p>
    </div>
  </div>
</template>
<style scoped>
.notification-page > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 28px;
}
h1 {
  font-size: 26px;
  margin: 0;
}
header p,
.empty-state p {
  color: var(--muted);
  font-size: 13px;
  margin: 10px 0 0;
}
.notification-grid {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  min-height: 420px;
  border: 1px solid var(--line);
  border-radius: 14px;
  overflow: hidden;
  background: var(--paper);
}
.notification-list {
  border-right: 1px solid var(--line);
}
.notification-list > button {
  display: grid;
  gap: 10px;
  text-align: left;
  width: 100%;
  padding: 20px;
  border: 0;
  border-bottom: 1px solid var(--line);
  background: transparent;
  color: var(--ink);
  cursor: pointer;
}
.notification-list > button:hover,
.notification-list > button.selected {
  background: var(--soft);
}
.item-heading {
  display: flex;
  align-items: center;
  gap: 8px;
}
.item-heading strong {
  font-size: 13px;
  font-weight: 500;
  overflow-wrap: anywhere;
}
.unread strong {
  font-weight: 650;
}
.item-heading i {
  width: 6px;
  height: 6px;
  flex-shrink: 0;
  background: var(--green);
  border-radius: 50%;
}
.item-heading svg {
  margin-left: auto;
  color: var(--muted);
}
small,
time {
  font-size: 11px;
  color: var(--muted);
}
article {
  padding: 36px;
  overflow-wrap: anywhere;
}
h2 {
  font-size: 22px;
  margin: 14px 0 24px;
}
.notification-body {
  font-size: 15px;
  line-height: 1.9;
  white-space: pre-wrap;
}
.detail-link {
  display: inline-block;
  margin-top: 18px;
  color: var(--green);
  font-size: 13px;
}
.empty-detail,
.empty-state {
  display: grid;
  place-content: center;
  text-align: center;
  color: var(--muted);
  padding: 60px;
}
.empty-state svg {
  margin: auto;
}
.pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px;
  font-size: 12px;
}
.pagination button {
  border: 0;
  background: transparent;
  color: var(--ink);
  cursor: pointer;
}
.pagination button:disabled {
  opacity: 0.35;
}
button:focus-visible {
  outline: 2px solid var(--green);
  outline-offset: -3px;
}
@media (max-width: 760px) {
  .notification-grid {
    grid-template-columns: 1fr;
  }
  .notification-list {
    border-right: 0;
  }
  article {
    padding: 24px;
  }
  .empty-detail {
    display: none;
  }
}
</style>
