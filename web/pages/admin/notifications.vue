<script setup lang="ts">
const confirmAction = useConfirm()
import type { NotificationList } from '~/shared/notifications'
definePageMeta({ layout: 'admin' })
useHead({ title: '站内通知 — AttackBox' })
const page = ref(1),
  form = ref({ title: '', body: '', link: '' }),
  busy = ref(false),
  message = ref('')
const { data, pending, error, refresh } = await useLazyFetch<{
  data: NotificationList
}>('/api/admin/notifications', { key: 'admin:notifications', query: { page } })
async function publish() {
  if (busy.value) return
  busy.value = true
  message.value = ''
  try {
    await $fetch('/api/admin/notifications', {
      method: 'POST',
      body: form.value,
      retry: false,
    })
    form.value = { title: '', body: '', link: '' }
    page.value = 1
    await refresh()
    message.value = '通知已发布'
  } catch (e: any) {
    message.value = e?.data?.message || '发布结果未确认，请先查看通知列表'
  } finally {
    busy.value = false
  }
}
async function withdraw(id: string) {
  if (busy.value) return
  if (!(await confirmAction({title:'撤回通知？',description:'撤回后，用户将无法再看到这条通知。',confirmLabel:'撤回通知'}))) return
  busy.value = true
  try {
    await $fetch(`/api/admin/notifications/${id}/withdraw`, {
      method: 'POST',
      retry: false,
    })
    await refresh()
    message.value = '通知已撤回'
  } catch (e: any) {
    message.value = e?.data?.message || '撤回失败'
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <div class="announcement-admin">
    <header>
      <h1>站内通知</h1>
      <p>发布课程更新与平台公告，向所有登录用户展示，也包括之后注册的用户。</p>
    </header>
    <p v-if="message" class="notice" role="status">{{ message }}</p>
    <div class="announcement-editor">
      <form @submit.prevent="publish">
        <h2>发布通知</h2>
        <label>
          标题
          <input
            :disabled="busy"
            v-model="form.title"
            required
            maxlength="120"
          />
        </label>
        <label>
          正文
          <textarea
            :disabled="busy"
            v-model="form.body"
            required
            maxlength="8000"
            rows="7"
          />
        </label>
        <label>
          相关页面
          <small>可选</small>
          <input
            :disabled="busy"
            v-model="form.link"
            maxlength="500"
            placeholder="/learn"
          />
        </label>
        <button
          class="primary"
          :disabled="busy || !form.title.trim() || !form.body.trim()"
        >
          <UiBusyLabel :busy="busy">发布通知</UiBusyLabel>
        </button>
      </form>
      <aside>
        <span class="eyebrow">用户阅读预览</span>
        <h2>{{ form.title || '通知标题' }}</h2>
        <p class="preview-body">{{ form.body || '正文将在这里展示。' }}</p>
        <span v-if="form.link" class="preview-link">查看相关内容</span>
      </aside>
    </div>
    <section class="announcement-history">
      <h2>已发布通知</h2>
      <p v-if="error" role="alert">列表读取失败</p>
      <UiBusyIndicator v-else-if="pending && !data" label="通知管理加载"/>
      <p v-else-if="!data?.data.items.length">暂无通知</p>
      <div v-for="item in data?.data.items" :key="item.id" class="history-row">
        <div>
          <strong>{{ item.title }}</strong>
          <small>
            {{item.sender.name}} · {{ item.publishedAt.slice(0, 10) }} ·
            {{ item.withdrawnAt ? '已撤回' : '展示中' }}
          </small>
        </div>
        <button
          v-if="!item.withdrawnAt"
          :disabled="busy"
          @click="withdraw(item.id)"
        >
          撤回
        </button>
      </div>
      <nav v-if="data && data.data.total > 20">
        <button :disabled="page === 1" @click="page--">上一页</button>
        <span>{{ page }}</span>
        <button :disabled="page * 20 >= data.data.total" @click="page++">
          下一页
        </button>
      </nav>
    </section>
  </div>
</template>
<style scoped>
h1 {
  font-size: 24px;
  margin: 0;
}
header > p {
  font-size: 12px;
  color: var(--muted);
  margin: 12px 0 24px;
}
.notice {
  padding: 14px;
  background: var(--soft);
  border-radius: 8px;
  font-size: 13px;
}
.announcement-editor {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(300px, 0.8fr);
  gap: 24px;
  align-items: start;
}
form,
aside,
.announcement-history {
  padding: 26px;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
}
h2 {
  font-size: 16px;
  margin: 0 0 24px;
}
label {
  display: block;
  font-size: 12px;
  margin-bottom: 20px;
}
small,
.eyebrow {
  font-size: 11px;
  color: var(--muted);
}
input,
textarea {
  display: block;
  width: 100%;
  box-sizing: border-box;
  margin-top: 9px;
  padding: 10px 12px;
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: 7px;
  color: var(--ink);
  font: inherit;
}
input {
  max-width: 500px;
}
textarea {
  resize: vertical;
  line-height: 1.7;
}
button {
  border: 1px solid var(--line);
  border-radius: 7px;
  background: var(--paper);
  color: var(--ink);
  padding: 9px 15px;
  cursor: pointer;
  font-size: 12px;
}
.primary {
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  border-color: transparent;
  box-shadow: var(--btn-primary-shadow);
  font-weight: 600;
}
.primary:hover:not(:disabled) {
  background: var(--btn-primary-hover);
}
button:disabled {
  opacity: 0.5;
}
.eyebrow {
  display: block;
  margin-bottom: 22px;
}
aside h2 {
  font-size: 21px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.preview-body {
  white-space: pre-wrap;
  line-height: 1.9;
  font-size: 14px;
  overflow-wrap: anywhere;
}
.preview-link {
  color: var(--green);
  font-size: 12px;
}
.announcement-history {
  margin-top: 24px;
}
.history-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 18px 0;
  border-top: 1px solid var(--line);
  font-size: 13px;
}
.history-row small {
  display: block;
  margin-top: 8px;
}
.announcement-history nav {
  display: flex;
  gap: 20px;
  align-items: center;
  justify-content: flex-end;
  margin-top: 20px;
}
</style>
