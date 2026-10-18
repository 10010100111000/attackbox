<template>
  <div
    v-if="visible"
    id="quick-notepad-window"
    ref="winRef"
    class="fixed z-[100000] flex flex-col bg-zinc-900 border border-zinc-700/80 rounded-xl shadow-2xl overflow-hidden text-zinc-200 select-none"
    :style="boxStyle"
  >
    <!-- 标题栏 (按住可拖动窗口) -->
    <div
      class="h-9 px-3 bg-zinc-900/90 border-b border-zinc-800 flex items-center justify-between cursor-move shrink-0"
      @pointerdown="onHeaderPointerDown"
    >
      <div class="flex items-center gap-2">
        <svg class="w-3.5 h-3.5 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2" />
          <rect x="8" y="2" width="8" height="4" rx="1" ry="1" />
          <path d="M9 12h6" />
          <path d="M9 16h4" />
        </svg>
        <span class="text-xs font-semibold text-zinc-100">快捷记事本</span>
      </div>
      <div class="flex items-center gap-1.5">
        <button
          type="button"
          @click="$emit('close')"
          class="w-5 h-5 rounded hover:bg-zinc-800 text-zinc-400 hover:text-zinc-100 flex items-center justify-center text-xs transition-colors cursor-pointer"
          title="关闭"
        >
          ✕
        </button>
      </div>
    </div>

    <!-- 顶部操作栏 -->
    <div class="p-2.5 bg-zinc-950/70 border-b border-zinc-800 flex flex-col gap-2 shrink-0">
      <div class="flex items-center gap-2">
        <div class="flex-1 flex items-center bg-zinc-900 border border-zinc-800 rounded-lg px-2.5 py-1.5 focus-within:border-zinc-600 transition-colors">
          <input
            v-model="inputVal"
            @keydown.enter.prevent="addSingle"
            type="text"
            placeholder="输入常用命令或文本行，按 Enter 添加..."
            class="w-full bg-transparent text-xs text-zinc-200 placeholder-zinc-500 font-mono outline-none"
          />
        </div>
        <button
          type="button"
          @click="addSingle"
          class="px-3 py-1.5 bg-zinc-100 text-zinc-900 hover:bg-white text-xs font-semibold rounded-lg transition-colors whitespace-nowrap cursor-pointer shadow-sm"
        >
          添加
        </button>
        <button
          type="button"
          @click="batchMode = !batchMode"
          :class="[
            'px-2.5 py-1.5 text-xs font-medium rounded-lg transition-colors whitespace-nowrap cursor-pointer',
            batchMode ? 'bg-zinc-800 text-white' : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/60'
          ]"
          title="展开批量导入文本框"
        >
          批量
        </button>
      </div>

      <!-- 批量导入展开区 -->
      <div v-if="batchMode" class="flex flex-col gap-2 pt-1">
        <textarea
          v-model="batchVal"
          rows="3"
          placeholder="在此粘贴多行文本，每行将自动拆分为独立行..."
          class="w-full bg-zinc-900 border border-zinc-800 rounded-lg p-2 text-xs text-zinc-200 placeholder-zinc-500 font-mono outline-none focus:border-zinc-600 resize-none"
        ></textarea>
        <div class="flex items-center justify-end gap-2">
          <button
            type="button"
            @click="confirmBatch"
            class="px-2.5 py-1 bg-zinc-100 hover:bg-white text-zinc-900 text-xs font-semibold rounded-md transition-colors cursor-pointer"
          >
            确认拆分导入
          </button>
          <button
            type="button"
            @click="batchMode = false"
            class="px-2.5 py-1 bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-xs rounded-md transition-colors cursor-pointer"
          >
            取消
          </button>
        </div>
      </div>
    </div>

    <!-- 文本行列表 (清爽无边框行，彻底杜绝 button 套 button) -->
    <div class="flex-1 overflow-y-auto divide-y divide-zinc-800/40 select-text">
      <div
        v-if="snippets.length === 0"
        class="h-36 flex flex-col items-center justify-center text-zinc-500 text-xs gap-1 select-none"
      >
        <span>暂无文本行</span>
        <span class="text-[11px] opacity-70">可在上方单行输入或批量粘贴导入</span>
      </div>

      <div
        v-for="(item, idx) in snippets"
        :key="idx"
        @click="copySnippet(item, idx)"
        :class="[
          'group flex items-center px-3 py-2 text-xs font-mono transition-colors cursor-pointer select-none',
          copiedIdx === idx ? 'bg-zinc-800/80 text-white' : 'hover:bg-zinc-800/40 text-zinc-300'
        ]"
        title="单击直接复制到剪贴板"
      >
        <!-- 行号 -->
        <span class="w-6 text-zinc-600 font-mono text-[11px] select-none shrink-0 text-left pr-2">{{ idx + 1 }}</span>

        <!-- 文本内容 -->
        <span class="flex-1 truncate select-text pr-2" :title="item">{{ item }}</span>

        <!-- 复制成功反馈 -->
        <span v-if="copiedIdx === idx" class="text-[11px] text-zinc-200 font-sans flex items-center gap-1 select-none shrink-0 font-medium">
          ✓ 已复制
        </span>

        <!-- 无边框操作图标组 (Hover 浮现，无任何厚重按钮框) -->
        <div v-else class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
          <button
            type="button"
            @click.stop="copySnippet(item, idx)"
            class="p-1 rounded text-zinc-400 hover:text-zinc-100 hover:bg-zinc-700/60 transition-colors cursor-pointer"
            title="复制到剪贴板"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
              <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
            </svg>
          </button>
          <button
            type="button"
            @click.stop="$emit('type', item)"
            class="p-1 rounded text-zinc-400 hover:text-zinc-100 hover:bg-zinc-700/60 transition-colors cursor-pointer"
            title="发送到当前活动窗口"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="4 17 10 11 4 5" />
              <line x1="12" y1="19" x2="20" y2="19" />
            </svg>
          </button>
          <button
            type="button"
            @click.stop="delSnippet(idx)"
            class="p-1 rounded text-zinc-500 hover:text-rose-400 hover:bg-rose-950/40 transition-colors cursor-pointer"
            title="删除行"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
      </div>
    </div>

    <!-- 底部状态与清空栏 -->
    <div class="h-7 px-3 bg-zinc-950/80 border-t border-zinc-800/80 flex items-center justify-between text-[11px] text-zinc-500 shrink-0">
      <span>单击任意行可直接复制</span>
      <div class="flex items-center gap-2">
        <span>{{ snippets.length }} 行</span>
        <button
          v-if="snippets.length > 0"
          type="button"
          @click="clearAll"
          class="text-zinc-500 hover:text-zinc-300 transition-colors cursor-pointer"
        >
          清空全部
        </button>
      </div>
    </div>

    <!-- 右下角拉伸缩放手柄 -->
    <div
      class="absolute right-0 bottom-0 w-3 h-3 cursor-nwse-resize"
      @pointerdown="onResizePointerDown"
      title="拖拽拉伸大小"
    ></div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const props = defineProps({
  visible: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['close', 'type'])

const STORAGE_KEY = 'attackbox_snippets_rows'
const snippets = ref([])
const inputVal = ref('')
const batchMode = ref(false)
const batchVal = ref('')
const copiedIdx = ref(-1)

// 窗口尺寸与定位状态
const winRef = ref(null)
const winX = ref(null)
const winY = ref(null)
const winW = ref(440)
const winH = ref(380)

const boxStyle = computed(() => {
  return {
    left: winX.value !== null ? `${winX.value}px` : 'auto',
    top: winY.value !== null ? `${winY.value}px` : 'auto',
    bottom: winY.value !== null ? 'auto' : '65px',
    right: winX.value !== null ? 'auto' : '20px',
    width: `${winW.value}px`,
    height: `${winH.value}px`
  }
})

onMounted(() => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      snippets.value = JSON.parse(raw)
    } else {
      snippets.value = [
        'id; whoami; hostname',
        'ip addr show eth0',
        'uname -a',
        'ss -tulpn'
      ]
      save()
    }
  } catch (_) {
    snippets.value = ['id; whoami; hostname']
  }
})

function save() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(snippets.value))
  } catch (_) {}
}

function addSingle() {
  const val = inputVal.value.trim()
  if (!val) return
  snippets.value.unshift(val)
  inputVal.value = ''
  save()
}

function confirmBatch() {
  const raw = batchVal.value
  if (!raw.trim()) return
  const lines = raw.split(/\r?\n/).map(l => l.trim()).filter(l => l.length > 0)
  if (lines.length > 0) {
    snippets.value.unshift(...lines)
    save()
    batchVal.value = ''
    batchMode.value = false
  }
}

function delSnippet(idx) {
  snippets.value.splice(idx, 1)
  save()
}

function clearAll() {
  if (confirm('确认清空所有记事本行内容吗？')) {
    snippets.value = []
    save()
  }
}

async function copySnippet(text, idx) {
  try {
    await navigator.clipboard.writeText(text)
    copiedIdx.value = idx
    setTimeout(() => {
      if (copiedIdx.value === idx) copiedIdx.value = -1
    }, 1000)
  } catch (_) {}
}

// ---------------- 标题栏拖拽逻辑 ----------------
let isDragging = false
let dragStartX = 0
let dragStartY = 0
let initBoxX = 0
let initBoxY = 0

function onHeaderPointerDown(e) {
  if (e.button !== 0 || e.target.closest('button')) return
  isDragging = true
  dragStartX = e.clientX
  dragStartY = e.clientY

  const rect = winRef.value.getBoundingClientRect()
  initBoxX = rect.left
  initBoxY = rect.top

  window.addEventListener('pointermove', onHeaderPointerMove)
  window.addEventListener('pointerup', onHeaderPointerUp)
  window.addEventListener('pointercancel', onHeaderPointerUp)
}

function onHeaderPointerMove(e) {
  if (!isDragging) return
  const dx = e.clientX - dragStartX
  const dy = e.clientY - dragStartY
  const maxL = window.innerWidth - winW.value - 8
  const maxT = window.innerHeight - winH.value - 8

  winX.value = Math.max(8, Math.min(initBoxX + dx, maxL))
  winY.value = Math.max(8, Math.min(initBoxY + dy, maxT))
}

function onHeaderPointerUp() {
  if (!isDragging) return
  isDragging = false
  window.removeEventListener('pointermove', onHeaderPointerMove)
  window.removeEventListener('pointerup', onHeaderPointerUp)
  window.removeEventListener('pointercancel', onHeaderPointerUp)
}

// ---------------- 右下角拉伸缩放逻辑 ----------------
let isResizing = false
let resizeStartX = 0
let resizeStartY = 0
let initW = 0
let initH = 0

function onResizePointerDown(e) {
  if (e.button !== 0) return
  isResizing = true
  resizeStartX = e.clientX
  resizeStartY = e.clientY
  initW = winW.value
  initH = winH.value

  window.addEventListener('pointermove', onResizePointerMove)
  window.addEventListener('pointerup', onResizePointerUp)
  window.addEventListener('pointercancel', onResizePointerUp)
}

function onResizePointerMove(e) {
  if (!isResizing) return
  const dx = e.clientX - resizeStartX
  const dy = e.clientY - resizeStartY
  winW.value = Math.max(300, Math.min(initW + dx, window.innerWidth - 30))
  winH.value = Math.max(220, Math.min(initH + dy, window.innerHeight - 30))
}

function onResizePointerUp() {
  if (!isResizing) return
  isResizing = false
  window.removeEventListener('pointermove', onResizePointerMove)
  window.removeEventListener('pointerup', onResizePointerUp)
  window.removeEventListener('pointercancel', onResizePointerUp)
}
</script>
