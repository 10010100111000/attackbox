<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useLazyFetch, useHead } from '#imports'
import { RefreshCw, Monitor } from 'lucide-vue-next'
import type { AdminSandboxInstance } from '~/shared/sandbox'

const confirmAction = useConfirm()
definePageMeta({ layout: 'admin' })
useHead({ title: '运行实例 — AttackBox' })
const { data, pending, error, refresh } = await useLazyFetch<{ data: AdminSandboxInstance[] }>(
  '/api/admin/instances',
  { key: 'admin:instances' }
)
const instances = computed(() => data.value?.data || [])
const stopping = ref<string[]>([]),
  notice = ref('')
const labels = {
  creating: '创建中',
  running: '运行中',
  stopping: '释放中',
  cleanup_failed: '清理待确认'
}
function date(value: string | null) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—'
}
async function stop(instance: AdminSandboxInstance) {
  if (
    stopping.value.includes(instance.id) ||
    !(await confirmAction({
      title: '释放用户环境？',
      description: `${instance.email || instance.userId} 的环境文件与进程会被清理。`,
      confirmLabel: '释放环境'
    }))
  )
    return
  stopping.value.push(instance.id)
  notice.value = ''
  try {
    await $fetch(`/api/admin/instances/${instance.id}/stop`, { method: 'POST' })
  } catch (error: any) {
    notice.value = error?.data?.message || '释放未确认，请刷新状态'
  } finally {
    stopping.value = stopping.value.filter(id => id !== instance.id)
    await refresh()
  }
}
let timer: ReturnType<typeof setInterval> | undefined
function poll() {
  if (!document.hidden) refresh()
}
onMounted(() => {
  timer = setInterval(poll, 10000)
  window.addEventListener('focus', poll)
})
onBeforeUnmount(() => {
  clearInterval(timer)
  window.removeEventListener('focus', poll)
})
</script>

<template>
  <div class="instances-page">
    <header>
      <div>
        <h1>运行实例</h1>
        <p>管理当前由新平台分配的环境。</p>
      </div>
      <UiButton
        variant="secondary"
        size="sm"
        :loading="pending"
        :disabled="pending"
        @click="refresh()"
        ><RefreshCw :size="14" aria-hidden="true" />刷新</UiButton
      >
    </header>
    <UiNotice v-if="notice || error" variant="error" class="mb-5">{{
      notice || '无法读取实例，请检查网络和管理员登录状态。'
    }}</UiNotice>
    <div v-if="instances.length" class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>用户 / 实例</th>
            <th>节点 / IP</th>
            <th>状态</th>
            <th>启动 / 到期</th>
            <th>最近活动</th>
            <th class="actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="instance in instances" :key="instance.id">
            <td>
              <strong>{{ instance.displayName || '已删除的用户' }}</strong
              ><span>{{ instance.email || instance.userId }}</span
              ><code>{{ instance.id }}</code>
            </td>
            <td>
              <strong>{{ instance.nodeId }}</strong
              ><span>{{ instance.ip || '等待分配' }}</span
              ><span
                >槽位 {{ instance.slot }} · {{ instance.policy.resources.memory_max_mb }} MB</span
              >
            </td>
            <td>
              <span class="state" :class="instance.state">{{ labels[instance.state] }}</span
              ><span v-if="!instance.connected" class="disconnected">节点断开</span
              ><span v-if="instance.error" class="instance-error">{{ instance.error }}</span>
            </td>
            <td>
              <span>{{ date(instance.startedAt) }}</span
              ><span>{{ date(instance.expiresAt) }}</span>
            </td>
            <td>{{ date(instance.lastActiveAt) }}</td>
            <td class="actions">
              <UiButton
                variant="danger"
                size="xs"
                :disabled="stopping.includes(instance.id) || instance.state === 'stopping'"
                @click="stop(instance)"
                >{{
                  stopping.includes(instance.id) || instance.state === 'stopping'
                    ? '释放中'
                    : instance.state === 'cleanup_failed'
                      ? '重试释放'
                      : '释放'
                }}</UiButton
              >
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <AdminDataLoading v-else-if="pending" :pending="pending" :has-data="!!data" />
    <UiEmptyState
      v-else-if="!error"
      title="当前没有实例"
      description="用户从学习页启动环境后，会在这里显示。"
      ><template #icon><Monitor :size="28" :stroke-width="1.5" aria-hidden="true" /></template
      ><template #action
        ><UiButton variant="secondary" size="sm" to="/admin/nodes">查看节点状态</UiButton></template
      ></UiEmptyState
    >
    <p class="footnote">
      关闭练习面板不会释放环境。释放结果以 Worker 确认为准；待确认时保留用户及槽位占用。
    </p>
  </div>
</template>

<style scoped>
header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 28px;
  gap: 24px;
}
h1 {
  font-size: 24px;
  font-weight: 750;
  margin: 0 0 6px;
}
header p {
  color: var(--muted);
  font-size: 13px;
  margin: 0;
}
.table-wrap {
  border: 1px solid var(--line);
  background: var(--paper);
  border-radius: 10px;
  overflow-x: auto;
}
table {
  width: 100%;
  min-width: 1050px;
  border-collapse: collapse;
  font-size: 12px;
  text-align: left;
}
th {
  color: var(--muted);
  background: var(--soft);
  font-weight: 500;
  font-size: 11px;
}
th,
td {
  padding: 16px 18px;
  border-bottom: 1px solid var(--line);
  vertical-align: middle;
}
tr:last-child td {
  border-bottom: 0;
}
td strong {
  display: block;
  font-weight: 550;
  font-size: 13px;
  margin-bottom: 4px;
}
td span,
td code {
  display: block;
  font-size: 11px;
  margin-top: 4px;
  color: var(--muted);
}
td code {
  font-family: ui-monospace, monospace;
}
td .state {
  color: var(--ink);
}
td .running {
  color: var(--success-ink);
}
td .disconnected,
td .instance-error,
td .cleanup_failed {
  color: var(--error-ink);
  max-width: 180px;
}
.actions {
  text-align: right;
  white-space: nowrap;
}
.footnote {
  font-size: 12px;
  line-height: 1.8;
  color: var(--muted);
  margin-top: 20px;
}
</style>
