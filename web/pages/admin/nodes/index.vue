<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useLazyFetch, useHead } from '#imports'
import { Server, RefreshCw, ChevronDown, SlidersHorizontal } from 'lucide-vue-next'
import type { SandboxNodesResponse, SandboxNodeSettings, SandboxNodeStatus } from '~/shared/sandbox'
definePageMeta({ layout: 'admin' })
useHead({ title: '节点管理 — AttackBox' })
const { data, pending, error, refresh } = await useLazyFetch<{ ok: boolean; data: SandboxNodesResponse }>('/api/admin/nodes', { key: 'admin:nodes' })
const status = computed(() => data.value?.data)
const editing = ref(''), draft = ref<SandboxNodeSettings | null>(null), saving = ref(false), settingsError = ref('')
function edit(node: SandboxNodeStatus) {
  if (saving.value) return
  editing.value = node.id; draft.value = { ...node.settings }; settingsError.value = ''
}
async function reloadSettings() {
  await refresh()
  const node = status.value?.nodes.find(n => n.id === editing.value)
  if (node) edit(node)
}
async function saveSettings() {
  if (!draft.value || saving.value) return
  saving.value = true; settingsError.value = ''
  try {
    await $fetch(`/api/admin/nodes/${editing.value}/settings`, { method: 'PUT', body: draft.value })
    editing.value = ''; draft.value = null; await refresh()
  } catch (error: any) { settingsError.value = error?.data?.message || '设置保存失败，请重试' }
  finally { saving.value = false }
}
const labels = { offline: '离线', registering: '正在注册', waiting: '等待心跳', online: '在线' }
const instanceLabels: Record<string, string> = { creating: '创建中', running: '运行中', stopping: '释放中', cleanup_failed: '清理失败' }
const online = computed(() => status.value?.nodes.filter(n => n.state === 'online').length || 0)
function date(value: string | null) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '尚未收到' }
function memory(mb: number) { return `${(mb / 1024).toFixed(1)} GB` }
let timer: ReturnType<typeof setInterval> | undefined
function poll() { if (!document.hidden) refresh() }
onMounted(() => { timer = setInterval(poll, 10000); window.addEventListener('focus', poll) })
onBeforeUnmount(() => { clearInterval(timer); window.removeEventListener('focus', poll) })
</script>

<template>
  <div class="nodes-page">
    <header class="page-head">
      <div>
        <h1>节点管理</h1>
        <p>查看 Worker 的连接、资源与实例清单。</p>
      </div>
      <UiButton variant="secondary" size="sm" :disabled="pending" @click="refresh()">
        <RefreshCw :size="14" aria-hidden="true" :class="{ 'animate-spin': pending }" />
        <UiBusyLabel :busy="pending">刷新</UiBusyLabel>
      </UiButton>
    </header>
    <UiNotice v-if="error" variant="error" class="mb-4">
      <div class="flex items-center justify-between gap-3 w-full">
        <span>节点状态加载失败，请检查网络或管理员登录状态。</span>
        <UiButton variant="secondary" size="xs" type="button" @click="refresh()">重试</UiButton>
      </div>
    </UiNotice>
    <AdminDataLoading v-else-if="!status" :pending="pending" :has-data="!!data" />
    <template v-if="status">
      <UiNotice v-if="status.configurationError" variant="error" class="mb-4">
        <strong>节点接入配置有误：</strong>
        <span>{{ status.configurationError }}。修正服务端配置后重启应用。</span>
      </UiNotice>
      <UiEmptyState
        v-else-if="!status.enabled"
        title="尚未启用节点接入"
        description="在服务端配置专用测试 Worker、集群身份及签名密钥后启用。详见 docs/implementation/sandbox-node-onboarding.md。"
      >
        <template #icon>
          <Server :size="28" :stroke-width="1.5" aria-hidden="true" />
        </template>
      </UiEmptyState>
      <template v-else>
        <div class="summary"><span><strong>{{ online }}</strong> / {{ status.nodes.length }} 节点在线</span><span class="observe-tag">{{ status.schedulingEnabled ? '实例调度已启用' : '仅接入观察 · 尚未开放调度' }}</span></div>
        <section v-for="node in status.nodes" :key="node.id" class="node-card" :aria-labelledby="`node-${node.id}`">
          <div class="node-head">
            <div class="node-identity"><Server :size="19" aria-hidden="true" /><div><h2 :id="`node-${node.id}`">{{ node.label }}</h2><code>{{ node.id }}</code></div></div>
            <div class="node-head-actions">
              <UiBadge v-if="node.settings.draining" variant="warning">排空中</UiBadge>
              <UiBadge :variant="node.state === 'online' ? 'success' : 'neutral'" dot :class="['node-state', node.state]">
                <UiBusyIndicator v-if="['registering','waiting'].includes(node.state)" inline :label="labels[node.state]"/>
                <template v-else>{{ labels[node.state] }}</template>
              </UiBadge>
              <UiButton variant="secondary" size="xs" type="button" :disabled="saving" :aria-expanded="editing === node.id" @click="edit(node)">
                <SlidersHorizontal :size="14" aria-hidden="true" />
                <span>设置</span>
              </UiButton>
            </div>
          </div>
          <div class="node-metrics" :class="{ stale: node.state !== 'online' }">
            <div><span>CPU 核心</span><strong>{{ node.capabilities?.cpuCores ?? '—' }}</strong></div>
            <div><span>可用 / 总内存</span><strong>{{ node.telemetry && node.capabilities ? `${memory(node.telemetry.availableMemoryMb)} / ${memory(node.capabilities.totalMemoryMb)}` : '—' }}</strong></div>
            <div><span>磁盘剩余</span><strong>{{ node.telemetry ? `${node.telemetry.freeDiskGb} GB` : '—' }}</strong></div>
            <div><span>系统负载 · 1 分钟</span><strong>{{ node.telemetry?.loadAverage.toFixed(2) ?? '—' }}</strong></div>
          </div>
          <dl class="node-info"><div><dt>数据地址</dt><dd><code>{{ node.publicDataUrl }}</code></dd></div><div><dt>最近心跳</dt><dd>{{ date(node.lastHeartbeatAt) }}<span v-if="node.lastHeartbeatAt && node.state !== 'online'" class="stale-label">历史数据</span></dd></div><div><dt>Worker</dt><dd>{{ node.version ? `${node.version} · ${node.capabilities?.os}` : '等待注册' }}</dd></div></dl>
          <form v-if="editing === node.id && draft" class="node-settings" @submit.prevent="saveSettings">
            <fieldset :disabled="saving"><div class="settings-title"><h3>节点设置</h3><label class="draining-switch"><input v-model="draft.draining" type="checkbox" />排空：停止分配新实例</label></div>
            <div class="settings-grid"><label class="name-field">节点名称<input v-model="draft.label" required maxlength="80" /></label><label>并发实例上限<input v-model.number="draft.maxSlots" type="number" min="1" max="250" required /></label><label>单实例内存 · MB<input v-model.number="draft.memoryMb" type="number" min="128" max="65536" required /></label><label>进程数上限<input v-model.number="draft.pidsMax" type="number" min="32" max="4096" required /></label></div>
            <p class="settings-help">资源用于新建实例；时限统一在运行策略中设置。降低并发上限不会主动终止实例。</p>
            <UiNotice v-if="settingsError" variant="error" class="mb-3">
              <div class="flex items-center justify-between gap-3 w-full">
                <span>{{ settingsError }}</span>
                <UiButton type="button" variant="secondary" size="xs" @click="reloadSettings">重新载入</UiButton>
              </div>
            </UiNotice>
            <div class="settings-actions">
              <UiButton type="button" variant="secondary" size="sm" @click="editing = ''; draft = null">取消</UiButton>
              <UiButton type="submit" variant="primary" size="sm" :loading="saving">保存设置</UiButton>
            </div></fieldset>
          </form>
          <details class="inventory"><summary><span>实例清单 <b>{{ node.sandboxes.length }}</b></span><ChevronDown :size="15" aria-hidden="true" /></summary><p v-if="!node.lastHeartbeatAt">收到完整心跳后显示。</p><p v-else-if="!node.sandboxes.length">最近心跳中没有运行实例。</p><template v-else><p class="inventory-note">{{ node.state === 'online' ? status.schedulingEnabled ? '当前 Worker 上报的实例清单，归属与释放操作见运行实例。' : '当前仅观察，不执行清理或调度。' : '以下为离线前的最后清单，不代表实例当前仍在运行。' }}</p><table><thead><tr><th>实例</th><th>状态</th><th>最近活动</th><th>GUI</th></tr></thead><tbody><tr v-for="box in node.sandboxes" :key="box.id"><td><code>{{ box.id }}</code></td><td>{{ instanceLabels[box.state] }}</td><td>{{ date(box.lastActiveAt) }}</td><td>{{ box.guiRunning ? '运行中' : '未启动' }}</td></tr></tbody></table></template></details>
        </section>
        <p class="footnote">状态每 10 秒刷新。连接状态为本次控制进程的观测值；查询不会延长沙箱寿命。</p>
      </template>
    </template>
  </div>
</template>

<style scoped>

.page-head, .node-head { display: flex; align-items: center; justify-content: space-between; gap: 24px; }
.page-head { margin-bottom: 28px; }
h1 { margin: 0 0 6px; font-size: 24px; font-weight: 750; }
.page-head p { color: var(--muted); font-size: 13px; }
.summary { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; color: var(--muted); font-size: 13px; }
.summary strong { color: var(--ink); font-size: 19px; }
.observe-tag { font-size: 12px; }
.node-card { background: var(--paper); border: 1px solid var(--line); border-radius: 10px; margin-bottom: 18px; overflow: hidden; }
.node-head { padding: 22px 24px; }
.node-identity { display: flex; align-items: center; gap: 13px; }
h2 { font-size: 16px; font-weight: 650; margin: 0 0 3px; }
code { font: 12px ui-monospace, monospace; color: var(--muted); overflow-wrap: anywhere; }
.node-metrics { display: grid; grid-template-columns: 1fr 1.5fr 1fr 1fr; margin: 0 24px; padding: 20px 0; border-top: 1px solid var(--line); border-bottom: 1px solid var(--line); gap: 20px; }
.node-metrics span { display: block; margin-bottom: 10px; color: var(--muted); font-size: 11px; }
.node-metrics strong { font-size: 18px; font-weight: 550; font-variant-numeric: tabular-nums; }
.stale { opacity: .6; }
.node-info { margin: 0; padding: 20px 24px; display: grid; grid-template-columns: 1.2fr 1.4fr 1fr; gap: 20px; font-size: 12px; }
dt { color: var(--muted); margin-bottom: 7px; } dd { margin: 0; overflow-wrap: anywhere; }
.stale-label { margin-left: 8px; color: var(--muted); font-size: 11px; }
.inventory { border-top: 1px solid var(--line); }
summary { list-style: none; padding: 14px 24px; display: flex; align-items: center; justify-content: space-between; font-size: 12px; cursor: pointer; }
summary::-webkit-details-marker { display: none; }
summary b { margin-left: 8px; color: var(--muted); font-weight: 500; }
.inventory[open] summary svg { transform: rotate(180deg); }
.inventory p { padding: 0 24px 18px; margin: 0; color: var(--muted); font-size: 12px; }
table { width: 100%; border-collapse: collapse; text-align: left; font-size: 12px; }
th, td { padding: 12px 24px; border-top: 1px solid var(--line); } th { color: var(--muted); font-weight: 500; }
.footnote { color: var(--muted); font-size: 12px; margin-top: 20px; }
.node-head-actions { display: flex; align-items: center; gap: 14px; }
.node-settings { padding: 22px 24px; border-top: 1px solid var(--line); background: var(--hover); }
fieldset { margin: 0; padding: 0; border: 0; min-width: 0; }
.settings-title { display: flex; align-items: center; justify-content: space-between; margin-bottom: 22px; }
h3 { font-size: 14px; font-weight: 600; margin: 0; }
.draining-switch { display: inline-flex; align-items: center; gap: 7px; font-size: 12px; color: var(--muted); }
.draining-switch input { accent-color: var(--green); }
.settings-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 20px 24px; }
.settings-grid label { display: flex; flex-direction: column; gap: 8px; color: var(--muted); font-size: 11px; }
.settings-grid input { width: 128px; padding: 8px 10px; color: var(--ink); background: var(--paper); border: 1px solid var(--line); border-radius: 5px; font-size: 13px; }
.settings-grid .name-field input { width: 100%; }
.settings-help { color: var(--muted); font-size: 11px; margin: 20px 0 0; line-height: 1.8; }
.settings-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 20px; }
fieldset:disabled input { opacity: .6; cursor: wait; }
</style>
