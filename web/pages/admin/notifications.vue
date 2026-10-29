<script setup lang="ts">
import type { NotificationList } from '~/shared/notifications'
import { apiErrorMessage, apiFetch } from '~/utils/api'

const confirmAction = useConfirm()
definePageMeta({ layout: 'admin' })
useHead({ title: '站内通知 — AttackBox' })
const page = ref(1),
  form = ref({ title: '', body: '', link: '' }),
  busy = ref(false),
  message = ref(''),
  failed = ref(false)
const { data, pending, error, refresh } = await useLazyFetch<{
  data: NotificationList
}>('/api/admin/notifications', { key: 'admin:notifications', query: { page } })
async function publish() {
  if (busy.value) return
  busy.value = true
  message.value = ''
  failed.value = false
  try {
    await apiFetch('/api/admin/notifications', {
      method: 'POST',
      body: form.value,
      retry: false
    })
    form.value = { title: '', body: '', link: '' }
    page.value = 1
    await refresh()
    message.value = '通知已发布'
  } catch (e) {
    failed.value = true
    message.value = apiErrorMessage(e, '发布结果未确认，请先查看通知列表')
  } finally {
    busy.value = false
  }
}
async function withdraw(id: string) {
  if (busy.value) return
  if (
    !(await confirmAction({
      title: '撤回通知？',
      description: '撤回后，用户将无法再看到这条通知。',
      confirmLabel: '撤回通知'
    }))
  )
    return
  busy.value = true
  message.value = ''
  failed.value = false
  try {
    await apiFetch(`/api/admin/notifications/${id}/withdraw`, {
      method: 'POST',
      retry: false
    })
    await refresh()
    message.value = '通知已撤回'
  } catch (e) {
    failed.value = true
    message.value = apiErrorMessage(e, '撤回失败')
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
    <UiNotice v-if="message" :variant="failed ? 'error' : 'success'" class="mb-6">{{
      message
    }}</UiNotice>
    <div class="announcement-editor">
      <form @submit.prevent="publish">
        <h2>发布通知</h2>
        <label>
          标题
          <UiInput v-model="form.title" bare :disabled="busy" required maxlength="120" />
        </label>
        <label>
          正文
          <UiTextarea v-model="form.body" :disabled="busy" required maxlength="8000" rows="7" />
        </label>
        <label>
          相关页面
          <small>可选</small>
          <UiInput v-model="form.link" bare :disabled="busy" maxlength="500" placeholder="/learn" />
        </label>
        <UiButton
          type="submit"
          variant="primary"
          :loading="busy"
          :disabled="busy || !form.title.trim() || !form.body.trim()"
        >
          发布通知
        </UiButton>
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
      <UiNotice v-if="error" variant="error" class="mb-4">列表读取失败</UiNotice>
      <UiBusyIndicator v-else-if="pending && !data" label="通知管理加载" />
      <UiEmptyState
        v-else-if="!data?.data.items.length"
        title="暂无通知"
        description="尚未发布任何站内公告通知。"
      />
      <div v-for="item in data?.data.items" :key="item.id" class="history-row">
        <div>
          <strong>{{ item.title }}</strong>
          <small class="flex items-center gap-2 mt-2">
            <span>{{ item.sender.name }} · {{ item.publishedAt.slice(0, 10) }}</span>
            <UiBadge :variant="item.withdrawnAt ? 'neutral' : 'success'" dot>
              {{ item.withdrawnAt ? '已撤回' : '展示中' }}
            </UiBadge>
          </small>
        </div>
        <UiButton
          v-if="!item.withdrawnAt"
          variant="secondary"
          size="xs"
          :disabled="busy"
          @click="withdraw(item.id)"
        >
          撤回
        </UiButton>
      </div>
      <nav v-if="data && data.data.total > 20">
        <UiButton variant="secondary" size="xs" :disabled="page === 1" @click="page--"
          >上一页</UiButton
        >
        <span class="text-xs text-muted">{{ page }}</span>
        <UiButton
          variant="secondary"
          size="xs"
          :disabled="page * 20 >= data.data.total"
          @click="page++"
        >
          下一页
        </UiButton>
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
  margin-top: 9px;
}
input {
  max-width: 500px;
}
textarea {
  resize: vertical;
  line-height: 1.7;
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
