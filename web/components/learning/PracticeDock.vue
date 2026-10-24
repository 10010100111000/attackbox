<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { Terminal, Monitor, Maximize2, Minimize2, Minus, X, Unplug, Plus, LoaderCircle } from 'lucide-vue-next'
import { useAuth } from '~/composables/useAuth'
import { LearningSandboxTerminal as SandboxTerminal, LearningSandboxDesktop as SandboxDesktop } from '#components'
import type { SandboxStatus, SandboxInstance } from '~/shared/sandbox'
const props = defineProps<{ active: boolean; mode: 'cli' | 'gui'; height: number; maxHeight: number; expanded: boolean }>()
const emit = defineEmits<{ 'update:height': [height: number]; 'update:expanded': [expanded: boolean]; minimize: []; close: [] }>()
const { currentUser } = useAuth()
const route = useRoute()
const sandbox = ref<SandboxStatus>({ available: false, instance: null })
const loading = ref(true), busy = ref(false), notice = ref('')
const desktopOpened = ref(false)
watch(() => props.mode, mode => {
  if (mode === 'gui') desktopOpened.value = true
  else if (sandbox.value.instance?.state === 'running' && !tabs.value.length) addTerminal()
}, { immediate: true })
const tabs = ref<{ id: string; label: string }[]>([]), selectedTab = ref('')
let pollTimer: ReturnType<typeof setInterval> | undefined, disposed = false, refreshGeneration = 0
function addTerminal() {
  const id = globalThis.crypto?.randomUUID?.() || `term-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 9)}`
  const tab = { id, label: `终端 ${++terminalCount}` }
  tabs.value.push(tab); selectedTab.value = tab.id
}
let terminalCount = 0
function closeTerminal(id: string) {
  const index = tabs.value.findIndex(t => t.id === id)
  tabs.value = tabs.value.filter(t => t.id !== id)
  if (selectedTab.value === id) selectedTab.value = tabs.value[Math.max(0, index - 1)]?.id || ''
}
watch(() => sandbox.value.instance?.id, (id, oldId) => {
  if (id === oldId) return
  tabs.value = []; selectedTab.value = ''; terminalCount = 0; desktopOpened.value = props.mode === 'gui'
  if (id && props.mode === 'cli' && sandbox.value.instance?.state === 'running') addTerminal()
})
watch(() => sandbox.value.instance?.state, state => { if (state === 'running' && props.mode === 'cli' && !tabs.value.length) addTerminal() })
async function refreshSandbox() {
  if (!currentUser.value || disposed || busy.value || document.hidden) { loading.value = false; return }
  const generation = ++refreshGeneration
  try {
    const response = await $fetch<{ data: SandboxStatus }>('/api/sandbox/status')
    if (disposed || generation !== refreshGeneration) return
    sandbox.value = response.data
  } catch (error: any) { if (!disposed && generation === refreshGeneration) notice.value = error?.data?.message || '无法获取环境状态' }
  finally { if (!disposed && generation === refreshGeneration) loading.value = false }
}
async function startSandbox() {
  if (busy.value) return
  busy.value = true; refreshGeneration++; notice.value = ''
  try {
    const response = await $fetch<{ data: SandboxInstance }>('/api/sandbox/start', { method: 'POST' })
    if (!disposed) sandbox.value = { available: true, instance: response.data }
  } catch (error: any) { if (!disposed) notice.value = error?.data?.message || '启动失败，请重试' }
  finally { busy.value = false; if (!disposed) await refreshSandbox() }
}
async function releaseSandbox() {
  const id = sandbox.value.instance?.id
  if (!id || busy.value || !confirm('释放当前环境？终端会关闭，环境中的文件和进程会被清理。')) return
  busy.value = true; refreshGeneration++; notice.value = ''
  try {
    const response = await $fetch<{ data: SandboxStatus }>('/api/sandbox/stop', { method: 'POST', body: { sandboxId: id } })
    if (!disposed) sandbox.value = response.data
  } catch (error: any) { if (!disposed) notice.value = error?.data?.message || '释放尚未确认，请刷新状态' }
  finally { busy.value = false; if (!disposed) await refreshSandbox() }
}
const stateLabels = { creating: '创建中', running: '运行中', stopping: '释放中', cleanup_failed: '等待清理' }
onMounted(() => { refreshSandbox(); pollTimer = setInterval(refreshSandbox, 10000); window.addEventListener('focus', refreshSandbox) })
watch(() => props.active, active => { if (active) refreshSandbox() })
onBeforeUnmount(() => { disposed = true; refreshGeneration++; clearInterval(pollTimer); window.removeEventListener('focus', refreshSandbox) })
const resizing = ref(false)
let startY = 0, startHeight = 0, grip: HTMLElement | null = null, pointerId: number | null = null
function setHeight(height: number) { emit('update:height', Math.max(180, Math.min(props.maxHeight, height))) }
function stopResize() {
  resizing.value = false
  if (grip && pointerId !== null && grip.hasPointerCapture(pointerId)) grip.releasePointerCapture(pointerId)
  grip = null; pointerId = null
}
function startResize(event: PointerEvent) {
  if (props.expanded || event.button !== 0) return
  event.preventDefault(); startY = event.clientY; startHeight = props.height
  grip = event.currentTarget as HTMLElement; pointerId = event.pointerId
  grip.setPointerCapture(event.pointerId); resizing.value = true
}
function moveResize(event: PointerEvent) { if (resizing.value) setHeight(startHeight + startY - event.clientY) }
function keyResize(event: KeyboardEvent) {
  const heights: Record<string, number> = { ArrowUp: props.height + 20, ArrowDown: props.height - 20, Home: 180, End: props.maxHeight }
  if (event.key in heights) { event.preventDefault(); setHeight(heights[event.key]!) }
}
watch(() => props.maxHeight, () => { if (props.height > props.maxHeight) setHeight(props.maxHeight) })
onBeforeUnmount(stopResize)
</script>

<template>
  <section class="practice-dock" :class="{ expanded, resizing }" :style="{ height: expanded ? undefined : `${height}px` }" aria-label="练习环境面板">
    <div v-if="!expanded" class="dock-grip" role="separator" tabindex="0" aria-label="调整练习面板高度" aria-orientation="horizontal" :aria-valuemin="180" :aria-valuemax="maxHeight" :aria-valuenow="height" @pointerdown="startResize" @pointermove="moveResize" @pointerup="stopResize" @pointercancel="stopResize" @lostpointercapture="stopResize" @keydown="keyResize"><span /></div>
    <div class="dock-header">
      <div class="dock-title"><Terminal v-if="mode === 'cli'" :size="15" aria-hidden="true" /><Monitor v-else :size="15" aria-hidden="true" />{{ mode === 'cli' ? '终端' : '桌面' }}<span v-if="sandbox.instance" class="sandbox-state">{{ sandbox.instance.connected ? stateLabels[sandbox.instance.state] : '节点断开' }}</span></div>
      <div class="dock-actions">
        <button v-if="sandbox.instance" type="button" class="release-btn" :disabled="busy" @click="releaseSandbox">释放环境</button>
        <button type="button" :aria-label="expanded ? '还原练习面板' : '展开练习面板'" @click="emit('update:expanded', !expanded)"><Minimize2 v-if="expanded" :size="15" aria-hidden="true" /><Maximize2 v-else :size="15" aria-hidden="true" /></button>
        <button type="button" aria-label="收起练习面板" @click="emit('minimize')"><Minus :size="16" aria-hidden="true" /></button>
        <button type="button" aria-label="关闭练习面板" @click="emit('close')"><X :size="16" aria-hidden="true" /></button>
      </div>
    </div>
    <p v-if="notice || sandbox.instance?.error" class="sandbox-notice" role="alert">{{ notice || sandbox.instance?.error }} <button type="button" @click="refreshSandbox">刷新状态</button></p>
    <div v-if="!currentUser" class="dock-empty"><Unplug :size="26" aria-hidden="true" /><h2>登录后使用练习环境</h2><NuxtLink :to="'/auth?returnTo=' + encodeURIComponent(route.fullPath)" class="sandbox-primary">登录并继续</NuxtLink></div>
    <div v-else-if="loading" class="dock-empty" role="status"><LoaderCircle :size="22" aria-hidden="true" /><p>正在获取环境状态…</p></div>
    <template v-else>
    <div v-if="sandbox.instance?.state === 'running' && desktopOpened" v-show="mode === 'gui'" class="desktop-area"><ClientOnly><SandboxDesktop :key="sandbox.instance.id" :sandbox-id="sandbox.instance.id" :active="active && mode === 'gui'" /></ClientOnly></div>
    <div v-if="sandbox.instance?.state === 'running'" v-show="mode === 'cli'" class="cli-area">
      <div class="terminal-tabs" aria-label="终端列表"><div v-for="tab in tabs" :key="tab.id" class="terminal-tab" :class="{ selected: selectedTab === tab.id }"><button type="button" :aria-pressed="selectedTab === tab.id" @click="selectedTab = tab.id">{{ tab.label }}</button><button type="button" :aria-label="'关闭' + tab.label" @click="closeTerminal(tab.id)"><X :size="12" aria-hidden="true" /></button></div><button type="button" class="add-terminal" aria-label="新建终端" :disabled="!sandbox.instance.connected" @click="addTerminal"><Plus :size="15" aria-hidden="true" /></button><span v-if="sandbox.instance.expiresAt" class="expiry">到期 {{ new Date(sandbox.instance.expiresAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}</span></div>
      <div class="terminal-workspace"><ClientOnly><SandboxTerminal v-for="tab in tabs" v-show="selectedTab === tab.id" :key="tab.id" :sandbox-id="sandbox.instance.id" :window-id="tab.id" :active="active && mode === 'cli' && selectedTab === tab.id" /></ClientOnly><div v-if="!tabs.length" class="dock-empty"><button type="button" class="sandbox-primary" @click="addTerminal">打开终端</button></div></div>
    </div>
    <div v-else-if="sandbox.instance" class="dock-empty" role="status"><LoaderCircle :size="22" aria-hidden="true" /><h2>{{ stateLabels[sandbox.instance.state] }}</h2><p>{{ sandbox.instance.state === 'cleanup_failed' ? '占用保留至节点确认释放，可重试释放环境。' : '状态将自动更新，你可以继续阅读。' }}</p><button type="button" class="sandbox-secondary" @click="refreshSandbox">刷新状态</button></div>
    <div v-else class="dock-empty"><Terminal :size="26" :stroke-width="1.5" aria-hidden="true" /><h2>{{ sandbox.available ? '准备开始实践' : '练习环境暂不可用' }}</h2><p>{{ sandbox.available ? '每个账号共用一份环境，可打开多个终端。' : '你可以继续阅读教程并完成章节练习。' }}</p><button v-if="sandbox.available" type="button" class="sandbox-primary" :disabled="busy" @click="startSandbox">{{ busy ? '正在启动…' : '启动环境' }}</button></div>
    </template>
  </section>
</template>

<style scoped>
.practice-dock { flex-shrink: 0; min-height: 180px; display: flex; flex-direction: column; border-top: 1px solid var(--line); background: var(--paper); z-index: 5; }
.practice-dock.expanded { position: absolute; inset: 45px 0 0; }
.dock-grip { height: 9px; flex-shrink: 0; display: grid; place-items: center; cursor: ns-resize; touch-action: none; }
.dock-grip > span { width: 36px; height: 3px; background: var(--line); border-radius: 3px; }
.dock-grip:hover > span, .resizing .dock-grip > span { background: var(--green); }
.dock-header { height: 38px; padding: 0 14px 0 20px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--line); flex-shrink: 0; }
.dock-title, .dock-actions { display: flex; align-items: center; gap: 8px; }
.dock-title { font-size: 12px; color: var(--ink); }
.dock-actions { gap: 3px; }
.dock-actions button { display: grid; place-items: center; width: 29px; height: 28px; padding: 0; border: 0; background: transparent; border-radius: 5px; color: var(--muted); cursor: pointer; }
.dock-actions button:hover { background: var(--soft); color: var(--ink); }
.dock-empty { flex: 1; min-height: 0; overflow: auto; display: flex; align-items: center; justify-content: center; flex-direction: column; padding: 20px; text-align: center; color: var(--muted); }
.dock-empty h2 { font-size: 14px; font-weight: 550; margin: 14px 0 6px; color: var(--ink); }
.dock-empty p { font-size: 12px; line-height: 1.7; margin: 0; }
button:focus-visible, .dock-grip:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
.sandbox-state { font-size: 10px; color: var(--muted); margin-left: 6px; }
.dock-actions .release-btn { width: auto; padding: 0 8px; font-size: 11px; }
.dock-actions .release-btn:hover { color: var(--error-ink); background: var(--error-bg); }
.sandbox-notice { margin: 0; padding: 8px 16px; font-size: 12px; color: var(--error-ink); background: var(--error-bg); }
.sandbox-notice button { margin-left: 8px; text-decoration: underline; background: transparent; border: 0; cursor: pointer; }
.sandbox-primary, .sandbox-secondary { display: inline-block; border-radius: 5px; padding: 9px 16px; font-size: 12px; text-decoration: none; border: 1px solid var(--line); cursor: pointer; }
.sandbox-primary { background: var(--green); color: var(--on-accent); border-color: var(--green); }
.sandbox-secondary { background: transparent; color: var(--ink); }
.sandbox-primary:disabled { opacity: .6; cursor: wait; }
.terminal-tabs { display: flex; align-items: center; gap: 4px; min-height: 32px; padding: 0 12px; border-bottom: 1px solid var(--line); overflow-x: auto; flex-shrink: 0; }
.terminal-tab { display: flex; align-items: center; gap: 3px; flex-shrink: 0; padding: 0 3px; border-bottom: 2px solid transparent; }
.terminal-tab.selected { border-bottom-color: var(--green); }
.terminal-tab button, .add-terminal { border: 0; background: transparent; color: var(--muted); cursor: pointer; display: flex; align-items: center; padding: 7px 4px; font-size: 11px; white-space: nowrap; }
.terminal-tab.selected button { color: var(--ink); }
.add-terminal:disabled { opacity: .5; cursor: not-allowed; }
.expiry { margin-left: auto; white-space: nowrap; color: var(--muted); font-size: 10px; }
.desktop-area { flex: 1; min-height: 0; }
.cli-area { flex: 1; min-height: 0; display: flex; flex-direction: column; }
.terminal-workspace { flex: 1; min-height: 0; }
</style>
