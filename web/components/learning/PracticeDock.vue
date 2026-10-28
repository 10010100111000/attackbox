<script setup lang="ts">
import { ref, computed, onBeforeUnmount, watch, useId } from 'vue'
import { Terminal, Monitor, ExternalLink, Minus, X, Unplug, Plus, Columns2, Rows2 } from 'lucide-vue-next'
import { useSandboxSession } from '~/composables/useSandboxSession'
import { LearningSandboxTerminal as SandboxTerminal, LearningSandboxDesktop as SandboxDesktop } from '#components'
const props = defineProps<{
  active: boolean; mode: 'cli' | 'gui'; placement: 'bottom' | 'right'; sideAvailable: boolean
  height: number; minHeight: number; maxHeight: number
  width: number; minWidth: number; maxWidth: number
}>()
const emit = defineEmits<{
  'update:height': [height: number]; 'update:width': [width: number]
  'update:placement': [placement: 'bottom' | 'right']; minimize: []; 'resize-end': []
}>()
const panelId = useId()
const isRight = computed(() => props.placement === 'right')
const size = computed(() => isRight.value ? props.width : props.height)
const minSize = computed(() => isRight.value ? props.minWidth : props.minHeight)
const maxSize = computed(() => isRight.value ? props.maxWidth : props.maxHeight)
const {sandbox,loading,notice,refresh:refreshSandbox,guiElsewhere,resumeGUI,openViewer,currentUser}=useSandboxSession()
const route = useRoute()
const desktopOpened = ref(false)
const tabs = ref<{ id: string; label: string }[]>([]), selectedTab = ref('')
function addTerminal() {
  const id = globalThis.crypto?.randomUUID?.() || `term-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 9)}`
  const tab = { id, label: `终端 ${++terminalCount}` }
  tabs.value.push(tab); selectedTab.value = tab.id
}
let terminalCount = 0
watch(() => props.mode, mode => {
  if (mode === 'gui') desktopOpened.value = true
  else if (sandbox.value.instance?.state === 'running' && !tabs.value.length) addTerminal()
}, { immediate: true })
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
const stateLabels = { creating: '创建中', running: '运行中', stopping: '释放中', cleanup_failed: '等待清理' }
watch(() => props.active, active => { if(active)void refreshSandbox() })
const resizing = ref(false)
let startPosition = 0, startSize = 0, grip: HTMLElement | null = null, pointerId: number | null = null
function setSize(value: number) {
  const bounded = Math.max(minSize.value, Math.min(maxSize.value, value))
  if (isRight.value) emit('update:width', bounded)
  else emit('update:height', bounded)
}
function stopResize() {
  const wasResizing = resizing.value
  resizing.value = false
  const element = grip, pointer = pointerId
  grip = null; pointerId = null
  if (element && pointer !== null && element.hasPointerCapture(pointer)) element.releasePointerCapture(pointer)
  if (wasResizing) emit('resize-end')
}
function startResize(event: PointerEvent) {
  if (event.button !== 0 || !props.active) return
  event.preventDefault()
  startPosition = isRight.value ? event.clientX : event.clientY
  startSize = size.value
  grip = event.currentTarget as HTMLElement; pointerId = event.pointerId
  grip.focus({ preventScroll: true })
  grip.setPointerCapture(event.pointerId); resizing.value = true
}
function moveResize(event: PointerEvent) {
  if (resizing.value && event.pointerId === pointerId) setSize(startSize + startPosition - (isRight.value ? event.clientX : event.clientY))
}
function keyResize(event: KeyboardEvent) {
  const grow = isRight.value ? 'ArrowLeft' : 'ArrowUp'
  const shrink = isRight.value ? 'ArrowRight' : 'ArrowDown'
  if (![grow, shrink, 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  setSize(event.key === 'Home' ? minSize.value : event.key === 'End' ? maxSize.value : size.value + (event.key === grow ? 20 : -20))
  emit('resize-end')
}
watch([() => props.placement, () => props.active], stopResize)
onBeforeUnmount(stopResize)

</script>

<template>
  <section :id="panelId" class="practice-dock" :class="{ resizing, 'dock-right': placement === 'right' }" :style="isRight ? { width: `${width}px` } : { height: `${height}px` }" aria-label="练习环境面板">
    <div class="dock-grip" role="separator" tabindex="0" :aria-label="isRight ? '调整练习面板宽度' : '调整练习面板高度'" :aria-orientation="isRight ? 'vertical' : 'horizontal'" :aria-controls="panelId" :aria-valuemin="minSize" :aria-valuemax="maxSize" :aria-valuenow="size" @pointerdown="startResize" @pointermove="moveResize" @pointerup="stopResize" @pointercancel="stopResize" @lostpointercapture="stopResize" @keydown="keyResize"><span /></div>
    <div class="dock-header">
      <div class="dock-title"><Terminal v-if="mode === 'cli'" :size="15" aria-hidden="true" /><Monitor v-else :size="15" aria-hidden="true" />{{ mode === 'cli' ? '终端' : '桌面' }}<span v-if="sandbox.instance" class="sandbox-state">{{ sandbox.instance.connected ? stateLabels[sandbox.instance.state] : '节点断开' }}</span></div>
      <div class="dock-actions">
        <button v-if="sideAvailable" type="button" class="placement-toggle" :title="placement === 'right' ? '切换为底部停靠' : '切换为右侧分屏'" :aria-label="placement === 'right' ? '切换为底部停靠' : '切换为右侧分屏'" @click="emit('update:placement', placement === 'right' ? 'bottom' : 'right')"><Rows2 v-if="placement === 'right'" :size="15" aria-hidden="true"/><Columns2 v-else :size="15" aria-hidden="true"/></button>
        <button type="button" class="independent-open" @click="openViewer(mode)" title="在新标签页打开" aria-label="在新标签页打开"><ExternalLink :size="15" aria-hidden="true"/></button>
        <button type="button" aria-label="收起练习面板" @click="emit('minimize')"><Minus :size="16" aria-hidden="true" /></button>
      </div>
    </div>
    <p v-if="notice || sandbox.instance?.error" class="sandbox-notice" role="alert">{{ notice || sandbox.instance?.error }}</p>
    <div v-if="!currentUser" class="dock-empty"><Unplug :size="26" aria-hidden="true" /><h2>登录后使用练习环境</h2><NuxtLink :to="'/auth?returnTo=' + encodeURIComponent(route.fullPath)" class="sandbox-primary">登录并继续</NuxtLink></div>
    <UiBusyIndicator v-else-if="loading" class="dock-empty" label="环境状态加载"/>
    <template v-else>
    <div v-if="sandbox.instance?.state === 'running' && desktopOpened && !guiElsewhere" v-show="mode === 'gui'" class="desktop-area"><ClientOnly><SandboxDesktop :key="sandbox.instance.id" :sandbox-id="sandbox.instance.id" :active="active && mode === 'gui'" :auto-resize="true" /></ClientOnly></div>
    <div v-if="sandbox.instance?.state === 'running' && mode==='gui' && guiElsewhere" class="dock-empty"><p>桌面已在其他页面打开</p><button @click="resumeGUI">在此继续操作</button></div>
    <div v-if="sandbox.instance?.state === 'running'" v-show="mode === 'cli'" class="cli-area">
      <div class="terminal-tabs" aria-label="终端列表"><div v-for="tab in tabs" :key="tab.id" class="terminal-tab" :class="{ selected: selectedTab === tab.id }"><button type="button" :aria-pressed="selectedTab === tab.id" @click="selectedTab = tab.id">{{ tab.label }}</button><button type="button" :aria-label="'关闭' + tab.label" @click="closeTerminal(tab.id)"><X :size="12" aria-hidden="true" /></button></div><button type="button" class="add-terminal" aria-label="新建终端" :disabled="!sandbox.instance.connected" @click="addTerminal"><Plus :size="15" aria-hidden="true" /></button></div>
      <div class="terminal-workspace"><ClientOnly><SandboxTerminal v-for="tab in tabs" v-show="selectedTab === tab.id" :key="tab.id" :sandbox-id="sandbox.instance.id" :window-id="tab.id" :active="active && mode === 'cli' && selectedTab === tab.id" /></ClientOnly><div v-if="!tabs.length" class="dock-empty"><button type="button" class="sandbox-primary" @click="addTerminal">打开终端</button></div></div>
    </div>
    <div v-else-if="sandbox.instance" class="dock-empty" role="status"><UiBusyIndicator v-if="sandbox.instance.state !== 'cleanup_failed'" label="环境准备"/><h2 v-else>{{stateLabels[sandbox.instance.state]}}</h2><p>{{ sandbox.instance.state === 'cleanup_failed' ? '占用保留至节点确认释放，可重试释放环境。' : '状态将自动更新，你可以继续阅读。' }}</p></div>
    <div v-else class="dock-empty"><Terminal :size="26" :stroke-width="1.5" aria-hidden="true" /><h2>{{ sandbox.available ? '准备开始实践' : '练习环境暂不可用' }}</h2><p>{{ sandbox.available ? '从正文顶部启动环境，再打开终端或桌面。' : '你可以继续阅读教程并完成章节练习。' }}</p></div>
    </template>
  </section>
</template>

<style scoped>
.practice-dock { position: relative; flex-shrink: 0; min-width: 0; min-height: 0; display: flex; flex-direction: column; border-top: 1px solid var(--line); background: var(--paper); z-index: 5; }
.practice-dock.dock-right { align-self: stretch; height: auto; border-top: 0; border-left: 1px solid var(--line); padding-left: 9px; }
.dock-grip { height: 9px; flex-shrink: 0; display: grid; place-items: center; cursor: ns-resize; touch-action: none; }
.dock-right .dock-grip { position: absolute; left: 0; top: 0; bottom: 0; width: 9px; height: auto; cursor: ew-resize; }
.dock-right .dock-grip > span { width: 3px; height: 36px; }
.dock-grip > span { width: 36px; height: 3px; background: var(--line); border-radius: 3px; }
.dock-grip:hover > span, .resizing .dock-right .dock-grip { position: absolute; left: 0; top: 0; bottom: 0; width: 9px; height: auto; cursor: ew-resize; }
.dock-right .dock-grip > span { width: 3px; height: 36px; }
.dock-grip > span { background: var(--green); }
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
.sandbox-notice { margin: 0; padding: 8px 16px; font-size: 12px; color: var(--error-ink); background: var(--error-bg); }
.sandbox-notice button { margin-left: 8px; text-decoration: underline; background: transparent; border: 0; cursor: pointer; }
.sandbox-primary, .sandbox-secondary { display: inline-block; border-radius: 5px; padding: 9px 16px; font-size: 12px; text-decoration: none; border: 1px solid var(--line); cursor: pointer; }
.sandbox-primary { background: var(--btn-primary-bg); color: var(--btn-primary-fg); border-color: transparent; box-shadow: var(--btn-primary-shadow); font-weight: 600; }
.sandbox-secondary { background: transparent; color: var(--ink); }
.sandbox-primary:disabled { opacity: .6; cursor: wait; }
.terminal-tabs { display: flex; align-items: center; gap: 4px; min-height: 32px; padding: 0 12px; border-bottom: 1px solid var(--line); overflow-x: auto; flex-shrink: 0; }
.terminal-tab { display: flex; align-items: center; gap: 3px; flex-shrink: 0; padding: 0 3px; border-bottom: 2px solid transparent; }
.terminal-tab.selected { border-bottom-color: var(--green); }
.terminal-tab button, .add-terminal { border: 0; background: transparent; color: var(--muted); cursor: pointer; display: flex; align-items: center; padding: 7px 4px; font-size: 11px; white-space: nowrap; }
.terminal-tab.selected button { color: var(--ink); }
.add-terminal:disabled { opacity: .5; cursor: not-allowed; }
.desktop-area { flex: 1; min-width: 0; min-height: 0; overflow: hidden; }
.cli-area { flex: 1; min-width: 0; min-height: 0; display: flex; flex-direction: column; }
.terminal-workspace { flex: 1; min-width: 0; min-height: 0; }
</style>
