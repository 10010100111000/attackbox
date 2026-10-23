<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useFetch, useHead } from '#imports'
import { RefreshCw, Monitor, CircleAlert } from 'lucide-vue-next'
import type { AdminSandboxInstance } from '~/shared/sandbox'
definePageMeta({ layout: 'admin' })
useHead({ title: '运行实例 — AttackBox' })
const { data, pending, error, refresh } = await useFetch<{ data: AdminSandboxInstance[] }>('/api/admin/instances', { key: 'admin:instances' })
const instances = computed(() => data.value?.data || [])
const stopping = ref<string[]>([]), notice = ref('')
const labels = { creating: '创建中', running: '运行中', stopping: '释放中', cleanup_failed: '清理待确认' }
function date(value: string | null) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—' }
async function stop(instance: AdminSandboxInstance) {
  if (stopping.value.includes(instance.id) || !confirm(`释放 ${instance.email || instance.userId} 的环境？环境中的文件与进程会被清理。`)) return
  stopping.value.push(instance.id); notice.value = ''
  try { await $fetch(`/api/admin/instances/${instance.id}/stop`, { method: 'POST' }) }
  catch (error: any) { notice.value = error?.data?.message || '释放未确认，请刷新状态' }
  finally { stopping.value = stopping.value.filter(id => id !== instance.id); await refresh() }
}
let timer: ReturnType<typeof setInterval> | undefined
function poll() { if (!document.hidden) refresh() }
onMounted(() => { timer = setInterval(poll, 10000); window.addEventListener('focus', poll) })
onBeforeUnmount(() => { clearInterval(timer); window.removeEventListener('focus', poll) })
</script>

<template>
  <div class="instances-page">
    <header><div><h1>运行实例</h1><p>管理当前由新平台分配的环境。</p></div><button class="refresh-btn" :disabled="pending" @click="refresh()"><RefreshCw :size="14" aria-hidden="true" />{{ pending ? '刷新中' : '刷新' }}</button></header>
    <div v-if="notice || error" class="notice" role="alert"><CircleAlert :size="16" aria-hidden="true" />{{ notice || '无法读取实例，请检查网络和管理员登录状态。' }}</div>
    <div v-if="instances.length" class="table-wrap"><table><thead><tr><th>用户 / 实例</th><th>节点 / IP</th><th>状态</th><th>启动 / 到期</th><th>最近活动</th><th class="actions">操作</th></tr></thead><tbody><tr v-for="instance in instances" :key="instance.id"><td><strong>{{ instance.displayName || '已删除的用户' }}</strong><span>{{ instance.email || instance.userId }}</span><code>{{ instance.id }}</code></td><td><strong>{{ instance.nodeId }}</strong><span>{{ instance.ip || '等待分配' }}</span><span>槽位 {{ instance.slot }} · {{ instance.policy.resources.memory_max_mb }} MB</span></td><td><span class="state" :class="instance.state">{{ labels[instance.state] }}</span><span v-if="!instance.connected" class="disconnected">节点断开</span><span v-if="instance.error" class="instance-error">{{ instance.error }}</span></td><td><span>{{ date(instance.startedAt) }}</span><span>{{ date(instance.expiresAt) }}</span></td><td>{{ date(instance.lastActiveAt) }}</td><td class="actions"><button type="button" class="stop-btn" :disabled="stopping.includes(instance.id) || instance.state === 'stopping'" @click="stop(instance)">{{ stopping.includes(instance.id) || instance.state === 'stopping' ? '释放中' : instance.state === 'cleanup_failed' ? '重试释放' : '释放' }}</button></td></tr></tbody></table></div>
    <div v-else-if="pending" class="empty" role="status">正在加载实例…</div>
    <div v-else-if="!error" class="empty"><Monitor :size="28" :stroke-width="1.5" aria-hidden="true" /><h2>当前没有实例</h2><p>用户从学习页启动环境后，会在这里显示。</p><NuxtLink to="/admin/nodes">查看节点状态</NuxtLink></div>
    <p class="footnote">关闭练习面板不会释放环境。释放结果以 Worker 确认为准；待确认时保留用户及槽位占用。</p>
  </div>
</template>

<style scoped>
.instances-page { max-width: 1400px; }
header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 28px; gap: 24px; }
h1 { font-size: 24px; font-weight: 750; margin: 0 0 6px; }
header p { color: var(--muted); font-size: 13px; margin: 0; }
.refresh-btn { display: inline-flex; align-items: center; gap: 7px; border: 1px solid var(--line); background: var(--paper); color: var(--ink); padding: 8px 14px; border-radius: 6px; font-size: 12px; cursor: pointer; }
.table-wrap { border: 1px solid var(--line); background: var(--paper); border-radius: 10px; overflow-x: auto; }
table { width: 100%; min-width: 1050px; border-collapse: collapse; font-size: 12px; text-align: left; }
th { color: var(--muted); background: var(--soft); font-weight: 500; font-size: 11px; }
th, td { padding: 16px 18px; border-bottom: 1px solid var(--line); vertical-align: middle; }
tr:last-child td { border-bottom: 0; }
td strong { display: block; font-weight: 550; font-size: 13px; margin-bottom: 4px; }
td span, td code { display: block; font-size: 11px; margin-top: 4px; color: var(--muted); }
td code { font-family: ui-monospace, monospace; }
td .state { color: var(--ink); }
td .running { color: var(--success-ink); }
td .disconnected, td .instance-error, td .cleanup_failed { color: var(--error-ink); max-width: 180px; }
.actions { text-align: right; white-space: nowrap; }
.stop-btn { padding: 7px 10px; border: 1px solid var(--line); background: transparent; color: var(--ink); border-radius: 5px; font-size: 12px; cursor: pointer; }
.stop-btn:hover { color: var(--error-ink); background: var(--error-bg); border-color: transparent; }
.stop-btn:disabled, .refresh-btn:disabled { opacity: .5; cursor: wait; }
.notice { display: flex; align-items: center; gap: 8px; font-size: 12px; background: var(--error-bg); color: var(--error-ink); padding: 14px; border-radius: 7px; margin-bottom: 20px; }
.empty { display: grid; justify-items: center; padding: 64px 24px; background: var(--paper); border: 1px dashed var(--line); border-radius: 10px; color: var(--muted); }
.empty h2 { font-size: 16px; font-weight: 550; margin: 16px 0 8px; color: var(--ink); }
.empty p, .empty a { font-size: 13px; } .empty a { margin-top: 16px; text-decoration: underline; color: var(--green); }
.footnote { font-size: 12px; line-height: 1.8; color: var(--muted); margin-top: 20px; }
</style>
