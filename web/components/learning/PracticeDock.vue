<script setup lang="ts">
import { ref, onBeforeUnmount, watch } from 'vue'
import { Terminal, Monitor, Maximize2, Minimize2, Minus, X, Unplug } from 'lucide-vue-next'
const props = defineProps<{ mode: 'cli' | 'gui'; height: number; maxHeight: number; expanded: boolean }>()
const emit = defineEmits<{ 'update:height': [height: number]; 'update:expanded': [expanded: boolean]; minimize: []; close: [] }>()
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
      <div class="dock-title"><Terminal v-if="mode === 'cli'" :size="15" aria-hidden="true" /><Monitor v-else :size="15" aria-hidden="true" />{{ mode === 'cli' ? '终端' : '桌面' }}</div>
      <div class="dock-actions">
        <button type="button" :aria-label="expanded ? '还原练习面板' : '展开练习面板'" @click="emit('update:expanded', !expanded)"><Minimize2 v-if="expanded" :size="15" aria-hidden="true" /><Maximize2 v-else :size="15" aria-hidden="true" /></button>
        <button type="button" aria-label="收起练习面板" @click="emit('minimize')"><Minus :size="16" aria-hidden="true" /></button>
        <button type="button" aria-label="关闭练习面板" @click="emit('close')"><X :size="16" aria-hidden="true" /></button>
      </div>
    </div>
    <div class="dock-empty"><Unplug :size="26" :stroke-width="1.5" aria-hidden="true" /><h2>练习环境暂不可用</h2><p>你可以继续阅读教程并完成章节练习。</p></div>
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
</style>
