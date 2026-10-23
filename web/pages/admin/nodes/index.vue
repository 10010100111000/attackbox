<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount } from 'vue'
import { useFetch, useHead } from '#imports'
import { Server, RefreshCw, CircleAlert, ChevronDown } from 'lucide-vue-next'
import type { SandboxNodesResponse } from '~/shared/sandbox'
definePageMeta({ layout: 'admin' })
useHead({ title: '节点管理 — AttackBox' })
const { data, pending, error, refresh } = await useFetch<{ ok: boolean; data: SandboxNodesResponse }>('/api/admin/nodes', { key: 'admin:nodes' })
const status = computed(() => data.value?.data)
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
    <header class="page-head"><div><h1>节点管理</h1><p>查看 Worker 的连接、资源与实例清单。</p></div><button class="refresh-btn" :disabled="pending" @click="refresh()"><RefreshCw :size="14" aria-hidden="true" />{{ pending ? '刷新中' : '刷新' }}</button></header>
    <div v-if="error" class="notice error" role="alert"><CircleAlert :size="17" aria-hidden="true" /><div>节点状态加载失败，请检查网络或管理员登录状态。<button @click="refresh()">重试</button></div></div>
    <p v-else-if="!status" class="empty" role="status">正在加载节点…</p>
    <template v-if="status">
      <div v-if="status.configurationError" class="notice error" role="alert"><CircleAlert :size="17" aria-hidden="true" /><div><strong>节点接入配置有误</strong><p>{{ status.configurationError }}。修正服务端配置后重启应用。</p></div></div>
      <div v-else-if="!status.enabled" class="empty"><Server :size="28" :stroke-width="1.5" aria-hidden="true" /><h2>尚未启用节点接入</h2><p>在服务端配置专用测试 Worker、集群身份及签名密钥后启用。</p><code>docs/implementation/sandbox-node-onboarding.md</code></div>
      <template v-else>
        <div class="summary"><span><strong>{{ online }}</strong> / {{ status.nodes.length }} 节点在线</span><span class="observe-tag">{{ status.schedulingEnabled ? '实例调度已启用' : '仅接入观察 · 尚未开放调度' }}</span></div>
        <section v-for="node in status.nodes" :key="node.id" class="node-card" :aria-labelledby="`node-${node.id}`">
          <div class="node-head"><div class="node-identity"><Server :size="19" aria-hidden="true" /><div><h2 :id="`node-${node.id}`">{{ node.label }}</h2><code>{{ node.id }}</code></div></div><span class="node-state" :class="node.state"><i aria-hidden="true" />{{ labels[node.state] }}</span></div>
          <div class="node-metrics" :class="{ stale: node.state !== 'online' }">
            <div><span>CPU 核心</span><strong>{{ node.capabilities?.cpuCores ?? '—' }}</strong></div>
            <div><span>可用 / 总内存</span><strong>{{ node.telemetry && node.capabilities ? `${memory(node.telemetry.availableMemoryMb)} / ${memory(node.capabilities.totalMemoryMb)}` : '—' }}</strong></div>
            <div><span>磁盘剩余</span><strong>{{ node.telemetry ? `${node.telemetry.freeDiskGb} GB` : '—' }}</strong></div>
            <div><span>系统负载 · 1 分钟</span><strong>{{ node.telemetry?.loadAverage.toFixed(2) ?? '—' }}</strong></div>
          </div>
          <dl class="node-info"><div><dt>数据地址</dt><dd><code>{{ node.publicDataUrl }}</code></dd></div><div><dt>最近心跳</dt><dd>{{ date(node.lastHeartbeatAt) }}<span v-if="node.lastHeartbeatAt && node.state !== 'online'" class="stale-label">历史数据</span></dd></div><div><dt>Worker</dt><dd>{{ node.version ? `${node.version} · ${node.capabilities?.os}` : '等待注册' }}</dd></div></dl>
          <details class="inventory"><summary><span>实例清单 <b>{{ node.sandboxes.length }}</b></span><ChevronDown :size="15" aria-hidden="true" /></summary><p v-if="!node.lastHeartbeatAt">收到完整心跳后显示。</p><p v-else-if="!node.sandboxes.length">最近心跳中没有运行实例。</p><template v-else><p class="inventory-note">{{ node.state === 'online' ? status.schedulingEnabled ? '当前 Worker 上报的实例清单，归属与释放操作见运行实例。' : '当前仅观察，不执行清理或调度。' : '以下为离线前的最后清单，不代表实例当前仍在运行。' }}</p><table><thead><tr><th>实例</th><th>状态</th><th>最近活动</th><th>GUI</th></tr></thead><tbody><tr v-for="box in node.sandboxes" :key="box.id"><td><code>{{ box.id }}</code></td><td>{{ instanceLabels[box.state] }}</td><td>{{ date(box.lastActiveAt) }}</td><td>{{ box.guiRunning ? '运行中' : '未启动' }}</td></tr></tbody></table></template></details>
        </section>
        <p class="footnote">状态每 10 秒刷新。连接状态为本次控制进程的观测值；查询不会延长沙箱寿命。</p>
      </template>
    </template>
  </div>
</template>

<style scoped>
.nodes-page { max-width: 1180px; }
.page-head, .node-head { display: flex; align-items: center; justify-content: space-between; gap: 24px; }
.page-head { margin-bottom: 28px; }
h1 { margin: 0 0 6px; font-size: 24px; font-weight: 750; }
.page-head p, .footnote { color: var(--muted); font-size: 13px; }
.refresh-btn { display: inline-flex; align-items: center; gap: 7px; border: 1px solid var(--line); background: var(--paper); color: var(--ink); border-radius: 6px; padding: 8px 14px; font-size: 12px; cursor: pointer; }
.refresh-btn:disabled { opacity: .6; cursor: wait; }
.summary { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; color: var(--muted); font-size: 13px; }
.summary strong { color: var(--ink); font-size: 19px; }
.observe-tag { font-size: 12px; }
.node-card { background: var(--paper); border: 1px solid var(--line); border-radius: 10px; margin-bottom: 18px; overflow: hidden; }
.node-head { padding: 22px 24px; }
.node-identity { display: flex; align-items: center; gap: 13px; }
h2 { font-size: 16px; font-weight: 650; margin: 0 0 3px; }
code { font: 12px ui-monospace, monospace; color: var(--muted); overflow-wrap: anywhere; }
.node-state { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; color: var(--muted); }
.node-state i { width: 6px; height: 6px; background: currentColor; border-radius: 50%; }
.node-state.online { color: var(--success-ink); }
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
.empty { padding: 64px 24px; text-align: center; border: 1px dashed var(--line); border-radius: 10px; background: var(--paper); }
.empty > svg { margin: 0 auto 18px; color: var(--muted); }
.empty p { margin: 10px 0 18px; font-size: 13px; color: var(--muted); }
.notice { display: flex; gap: 12px; padding: 18px; border-radius: 8px; font-size: 13px; margin-bottom: 20px; }
.notice p { margin: 6px 0 0; } .notice button { text-decoration: underline; margin-left: 12px; }
.error { background: var(--error-bg); color: var(--error-ink); }
.footnote { font-size: 12px; margin-top: 20px; }
</style>
