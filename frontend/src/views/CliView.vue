<template>
  <div class="relative w-screen h-screen overflow-hidden bg-zinc-950 flex flex-col select-none text-zinc-200">
    <!-- 顶部极简现代悬浮控制栏 (HUD Island，无多余装饰，纯功能按钮，保持 dash.html 黑白灰风格) -->
    <header class="fixed top-3 left-1/2 -translate-x-1/2 z-40 h-10 px-3 bg-zinc-900/90 border border-zinc-700/80 backdrop-blur-md rounded-full shadow-2xl flex items-center gap-2">
      <!-- 新建终端 -->
      <button
        type="button"
        @click="createTerminal"
        class="px-3 py-1 rounded-full text-xs font-medium bg-zinc-100 hover:bg-white text-zinc-950 flex items-center gap-1.5 shadow-sm transition-colors cursor-pointer"
        title="新建独立命令行窗口 (Ctrl+Alt+T)"
      >
        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <line x1="12" y1="5" x2="12" y2="19"></line>
          <line x1="5" y1="12" x2="19" y2="12"></line>
        </svg>
        <span>新建终端</span>
      </button>

      <!-- 平铺排列 -->
      <button
        type="button"
        @click="tileWindows"
        class="px-3 py-1 rounded-full text-xs font-medium bg-zinc-800 hover:bg-zinc-700 text-zinc-200 hover:text-white border border-zinc-700 flex items-center gap-1.5 transition-colors cursor-pointer"
        title="整齐平铺所有打开的终端窗口"
      >
        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="3" y="3" width="7" height="7"></rect>
          <rect x="14" y="3" width="7" height="7"></rect>
          <rect x="14" y="14" width="7" height="7"></rect>
          <rect x="3" y="14" width="7" height="7"></rect>
        </svg>
        <span>平铺排列</span>
      </button>

      <!-- 阶梯排列 -->
      <button
        type="button"
        @click="cascadeWindows"
        class="px-3 py-1 rounded-full text-xs font-medium bg-zinc-800 hover:bg-zinc-700 text-zinc-200 hover:text-white border border-zinc-700 flex items-center gap-1.5 transition-colors cursor-pointer"
        title="经典阶梯层叠错开排列窗口"
      >
        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="2" y="2" width="13" height="13" rx="2"></rect>
          <path d="M7 17v3a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3"></path>
        </svg>
        <span>阶梯排列</span>
      </button>

      <!-- 关闭全部 -->
      <button
        v-if="termCount > 0"
        type="button"
        @click="closeAllWindows"
        class="px-2.5 py-1 rounded-full text-xs font-medium text-zinc-400 hover:text-rose-400 hover:bg-rose-950/40 transition-colors ml-1 cursor-pointer"
        title="关闭当前所有终端"
      >
        全部关闭
      </button>
    </header>

    <!-- 虚拟桌面空白占位指引 (无终端打开时显示) -->
    <main class="w-full h-full flex flex-col items-center justify-center pointer-events-none select-none">
      <div v-if="termCount === 0" class="pointer-events-auto flex flex-col items-center justify-center gap-3 text-zinc-500">
        <svg class="w-10 h-10 text-zinc-700" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="4 17 10 11 4 5"></polyline>
          <line x1="12" y1="19" x2="20" y2="19"></line>
        </svg>
        <p class="text-xs">当前工作区暂无打开的命令行终端</p>
        <button
          type="button"
          @click="createTerminal"
          class="px-4 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-xs text-zinc-200 hover:bg-zinc-800 hover:text-white flex items-center gap-1.5 transition-colors shadow-sm cursor-pointer"
        >
          <svg class="w-3.5 h-3.5 text-zinc-300" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="12" y1="5" x2="12" y2="19"></line>
            <line x1="5" y1="12" x2="19" y2="12"></line>
          </svg>
          <span>新建第一个终端</span>
        </button>
      </div>
    </main>

    <!-- 自由拖拽浮动 Dock 胶囊 (超高 z-index 保证置顶) -->
    <DraggableDock
      storage-key="attackbox_term_dock_pos_v2"
      :active="notepadVisible"
      @toggle="notepadVisible = !notepadVisible"
    />

    <!-- 快捷记事本弹窗组件 (超高 z-index 保证置顶) -->
    <QuickNotepad
      :visible="notepadVisible"
      @close="notepadVisible = false"
      @type="onTypeSnippetToTerminal"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import DraggableDock from '../components/DraggableDock.vue'
import QuickNotepad from '../components/QuickNotepad.vue'

const termCount = ref(0)
const focusedNum = ref(null)
const notepadVisible = ref(false)

let termSeq = 0
const activeBoxes = []

function getTermCtor() {
  if (typeof window.Terminal === 'function') return window.Terminal
  if (window.Terminal && typeof window.Terminal.Terminal === 'function') return window.Terminal.Terminal
  return null
}

function getFitCtor() {
  if (typeof window.FitAddon === 'function') return window.FitAddon
  if (window.FitAddon && typeof window.FitAddon.FitAddon === 'function') return window.FitAddon.FitAddon
  return null
}

function createTerminal() {
  if (typeof window.WinBox === 'undefined') {
    console.error('WinBox 未就绪')
    return
  }

  termSeq++
  const num = termSeq

  const mountEl = document.createElement('div')
  mountEl.className = 'w-full h-full bg-[#09090b] overflow-hidden p-2'

  const vw = window.innerWidth
  const vh = window.innerHeight
  const initW = Math.min(Math.max(vw * 0.54, 520), 840)
  const initH = Math.min(Math.max(vh * 0.56, 320), 520)
  const offset = (activeBoxes.length % 7) * 28

  const x = Math.max(30 + offset, 16)
  const y = Math.max(65 + offset, 60)

  let term = null
  let fit = null
  let ws = null

  const wb = new window.WinBox({
    title: `终端 #${num}`,
    class: ['dark', 'term-box', 'no-full'],
    x: x,
    y: y,
    width: `${initW}px`,
    height: `${initH}px`,
    minwidth: 360,
    minheight: 220,
    top: 56, // 避开顶部控制栏
    bottom: 10,
    left: 10,
    right: 10,
    mount: mountEl,
    onfocus: () => {
      focusedNum.value = num
      if (term) term.focus()
    },
    onresize: () => {
      if (fit) {
        try { fit.fit() } catch (_) {}
      }
      syncSize()
    },
    onclose: () => {
      const idx = activeBoxes.findIndex(b => b.num === num)
      if (idx !== -1) {
        const item = activeBoxes[idx]
        try {
          if (item.ws) {
            item.ws.onclose = null
            item.ws.close()
          }
          if (item.term) item.term.dispose()
        } catch (_) {}
        activeBoxes.splice(idx, 1)
        termCount.value = activeBoxes.length
      }
    }
  })

  // 初始化 xterm 终端
  const TermCtor = getTermCtor()
  if (!TermCtor) return

  term = new TermCtor({
    cursorBlink: true,
    cursorStyle: 'block',
    scrollback: 5000,
    overviewRulerWidth: 0,
    fontFamily: "'JetBrains Mono', ui-monospace, Menlo, Consolas, monospace",
    fontSize: 13,
    lineHeight: 1.25,
    theme: {
      background: '#09090b',
      foreground: '#f4f4f5',
      cursor: '#ffffff',
      cursorAccent: '#09090b',
      selectionBackground: 'rgba(255, 255, 255, 0.2)',
      black: '#18181b', red: '#ef4444', green: '#22c55e', yellow: '#eab308',
      blue: '#3b82f6', magenta: '#a855f7', cyan: '#06b6d4', white: '#f4f4f5',
      brightBlack: '#71717a', brightRed: '#f87171', brightGreen: '#4ade80',
      brightYellow: '#fde047', brightBlue: '#60a5fa', brightMagenta: '#c084fc',
      brightCyan: '#22d3ee', brightWhite: '#ffffff'
    }
  })

  const FitCtor = getFitCtor()
  if (FitCtor) {
    try {
      fit = new FitCtor()
      term.loadAddon(fit)
    } catch (_) {}
  }

  term.open(mountEl)

  const scheme = location.protocol === 'https:' ? 'wss://' : 'ws://'
  ws = new WebSocket(`${scheme}${location.host}/term/ws?win=w${num}`)
  ws.binaryType = 'arraybuffer'

  function syncSize() {
    if (ws && ws.readyState === WebSocket.OPEN && term.cols > 0 && term.rows > 0) {
      ws.send(JSON.stringify({ cols: term.cols, rows: term.rows }))
    }
  }

  ws.onopen = () => {
    if (fit) {
      try { fit.fit() } catch (_) {}
    }
    syncSize()
    term.focus()
  }

  ws.onmessage = (e) => {
    if (typeof e.data !== 'string') {
      term.write(new Uint8Array(e.data))
    }
  }

  ws.onclose = (e) => {
    if (e.code === 4001) {
      term.write('\r\n\x1b[33m[AttackBox] 窗口会话已被新连接替换接管。\x1b[0m\r\n')
    } else {
      term.write('\r\n\x1b[90m[AttackBox] 终端会话已断开。\x1b[0m\r\n')
    }
  }

  term.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(new TextEncoder().encode(data))
    }
  })

  const boxItem = { num, winbox: wb, term, fit, ws, mountEl }
  activeBoxes.push(boxItem)
  termCount.value = activeBoxes.length
  focusedNum.value = num

  setTimeout(() => {
    if (fit) {
      try { fit.fit() } catch (_) {}
    }
    syncSize()
    term.focus()
  }, 100)
}

function tileWindows() {
  const count = activeBoxes.length
  if (count === 0) return

  const vw = window.innerWidth
  const vh = window.innerHeight
  const topH = 60
  const gap = 10

  let cols = 1, rows = 1
  if (count === 2) { cols = 2; rows = 1 }
  else if (count <= 4) { cols = 2; rows = 2 }
  else if (count <= 6) { cols = 3; rows = 2 }
  else { cols = 3; rows = 3 }

  const totalW = vw - gap * (cols + 1)
  const totalH = vh - topH - gap * (rows + 1)
  const w = Math.floor(totalW / cols)
  const h = Math.floor(totalH / rows)

  activeBoxes.forEach((item, idx) => {
    const c = idx % cols
    const r = Math.floor(idx / cols)
    const left = gap + c * (w + gap)
    const top = topH + gap + r * (h + gap)

    item.winbox.resize(w, h).move(left, top)
    if (item.fit) {
      setTimeout(() => {
        try { item.fit.fit() } catch (_) {}
      }, 50)
    }
  })
}

function cascadeWindows() {
  const count = activeBoxes.length
  if (count === 0) return

  const vw = window.innerWidth
  const vh = window.innerHeight
  const initW = Math.min(Math.max(vw * 0.54, 520), 840)
  const initH = Math.min(Math.max(vh * 0.56, 320), 520)

  activeBoxes.forEach((item, idx) => {
    const left = Math.max(24 + idx * 32, 16)
    const top = Math.max(68 + idx * 26, 60)
    item.winbox.resize(initW, initH).move(left, top).focus()
    if (item.fit) {
      setTimeout(() => {
        try { item.fit.fit() } catch (_) {}
      }, 50)
    }
  })
}

function closeAllWindows() {
  const copy = [...activeBoxes]
  copy.forEach(item => {
    try { item.winbox.close() } catch (_) {}
  })
  activeBoxes.length = 0
  termCount.value = 0
}

function onTypeSnippetToTerminal(text) {
  if (!text) return
  let target = null
  if (focusedNum.value) {
    target = activeBoxes.find(b => b.num === focusedNum.value)
  }
  if (!target && activeBoxes.length > 0) {
    target = activeBoxes[0]
  }

  if (target && target.ws && target.ws.readyState === WebSocket.OPEN) {
    const payload = text.endsWith('\n') ? text : text + '\n'
    target.ws.send(new TextEncoder().encode(payload))
    target.term.focus()
  }
}

function onKeydown(e) {
  if (e.ctrlKey && e.altKey && e.key.toLowerCase() === 't') {
    e.preventDefault()
    createTerminal()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  // 默认启动一个主终端
  createTerminal()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  closeAllWindows()
})
</script>

<style>
/* WinBox 定制暗黑风格：深度契合 dash.html 经典黑白灰 */
.winbox.term-box {
  background: #18181b !important;
  border-radius: 8px !important;
  overflow: hidden !important;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.7) !important;
  border: 1px solid #3f3f46 !important;
}

.winbox.term-box.focus {
  border-color: #71717a !important;
  box-shadow: 0 25px 50px rgba(0, 0, 0, 0.85) !important;
}

.winbox.term-box .wb-header {
  background: #121215 !important;
  border-bottom: 1px solid #27272a !important;
  height: 36px !important;
  line-height: 36px !important;
  padding: 0 10px !important;
}

.winbox.term-box .wb-title {
  font-family: 'JetBrains Mono', ui-monospace, monospace !important;
  font-size: 12px !important;
  color: #f4f4f5 !important;
  font-weight: 600 !important;
  letter-spacing: -0.01em !important;
}

.winbox.term-box .wb-control {
  opacity: 0.8 !important;
}

.winbox.term-box .wb-control:hover {
  opacity: 1 !important;
}

/* 彻底隐藏全屏按钮 */
.winbox.term-box .wb-full {
  display: none !important;
}

/* 彻底消除外层多余的浏览器原生滚动条 */
.winbox.term-box .wb-body {
  background: #09090b !important;
  overflow: hidden !important;
}

/* xterm.js 窗口内部滚动条定制：仅在超出需要滚动时才展现优雅细条 */
.winbox.term-box .xterm .xterm-viewport {
  overflow-y: auto !important;
  scrollbar-width: thin !important;
  scrollbar-color: #27272a transparent !important;
}

.winbox.term-box .xterm .xterm-viewport::-webkit-scrollbar {
  width: 4px !important;
}

.winbox.term-box .xterm .xterm-viewport::-webkit-scrollbar-track {
  background: transparent !important;
}

.winbox.term-box .xterm .xterm-viewport::-webkit-scrollbar-thumb {
  background: #27272a !important;
  border-radius: 4px !important;
}

.winbox.term-box .xterm .xterm-viewport::-webkit-scrollbar-thumb:hover {
  background: #3f3f46 !important;
}
</style>
