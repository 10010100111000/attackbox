<template>
  <div
    ref="dockRef"
    class="fixed z-[99999] flex items-center gap-1.5 p-1 bg-zinc-900/90 border border-zinc-700/80 backdrop-blur-md rounded-full shadow-2xl text-zinc-200 select-none transition-shadow hover:border-zinc-500"
    :style="posStyle"
  >
    <!-- 拖动手柄 (仅此区域响应拖拽) -->
    <div
      class="px-1.5 py-1 text-zinc-400 hover:text-zinc-200 cursor-grab active:cursor-grabbing flex items-center justify-center"
      title="按住手柄拖动位置"
      @pointerdown="onPointerDown"
    >
      <svg class="w-2.5 h-3.5 fill-current opacity-70" viewBox="0 0 8 12">
        <circle cx="2" cy="2" r="1.2" />
        <circle cx="6" cy="2" r="1.2" />
        <circle cx="2" cy="6" r="1.2" />
        <circle cx="6" cy="6" r="1.2" />
        <circle cx="2" cy="10" r="1.2" />
        <circle cx="6" cy="10" r="1.2" />
      </svg>
    </div>

    <!-- 记事本触发按钮 (纯原生 click，不受拖拽干扰) -->
    <button
      type="button"
      @click.stop="$emit('toggle')"
      :class="[
        'px-2.5 py-1 rounded-full text-xs font-medium flex items-center gap-1.5 transition-colors cursor-pointer',
        active ? 'bg-zinc-800 text-white border border-zinc-500' : 'bg-zinc-800/80 text-zinc-300 hover:bg-zinc-700 hover:text-white border border-zinc-700'
      ]"
      title="打开/关闭快捷记事本"
    >
      <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2" />
        <rect x="8" y="2" width="8" height="4" rx="1" ry="1" />
        <path d="M9 12h6" />
        <path d="M9 16h4" />
      </svg>
      <span>记事本</span>
    </button>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const props = defineProps({
  storageKey: {
    type: String,
    default: 'attackbox_dock_pos'
  },
  active: {
    type: Boolean,
    default: false
  }
})

defineEmits(['toggle'])

const dockRef = ref(null)
const left = ref(null)
const top = ref(null)

const posStyle = computed(() => {
  if (left.value !== null && top.value !== null) {
    return {
      left: `${left.value}px`,
      top: `${top.value}px`,
      right: 'auto',
      bottom: 'auto'
    }
  }
  return {
    bottom: '20px',
    right: '20px'
  }
})

onMounted(() => {
  try {
    const saved = localStorage.getItem(props.storageKey)
    if (saved) {
      const pos = JSON.parse(saved)
      if (typeof pos.left === 'number' && typeof pos.top === 'number') {
        const maxL = window.innerWidth - 130
        const maxT = window.innerHeight - 45
        left.value = Math.max(8, Math.min(pos.left, maxL))
        top.value = Math.max(8, Math.min(pos.top, maxT))
      }
    }
  } catch (_) {}
})

let isDragging = false
let startPointerX = 0
let startPointerY = 0
let initLeft = 0
let initTop = 0

function onPointerDown(e) {
  if (e.button !== 0) return
  isDragging = true
  startPointerX = e.clientX
  startPointerY = e.clientY

  const rect = dockRef.value.getBoundingClientRect()
  initLeft = rect.left
  initTop = rect.top

  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerUp)
}

function onPointerMove(e) {
  if (!isDragging) return
  const dx = e.clientX - startPointerX
  const dy = e.clientY - startPointerY
  const maxL = window.innerWidth - (dockRef.value?.offsetWidth || 130)
  const maxT = window.innerHeight - (dockRef.value?.offsetHeight || 45)

  left.value = Math.max(8, Math.min(initLeft + dx, maxL))
  top.value = Math.max(8, Math.min(initTop + dy, maxT))
}

function onPointerUp() {
  if (!isDragging) return
  isDragging = false
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerUp)

  try {
    localStorage.setItem(props.storageKey, JSON.stringify({ left: left.value, top: top.value }))
  } catch (_) {}
}
</script>
