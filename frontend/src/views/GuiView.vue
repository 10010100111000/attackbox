<template>
  <div class="relative w-screen h-screen overflow-hidden bg-zinc-950 flex flex-col select-none">
    <!-- noVNC 远程桌面挂载视口 -->
    <div ref="screenRef" class="w-full h-full flex items-center justify-center outline-none"></div>

    <!-- 极简自由拖拽 Dock 胶囊 -->
    <DraggableDock
      storage-key="attackbox_gui_dock_pos_v2"
      :active="notepadVisible"
      @toggle="notepadVisible = !notepadVisible"
    />

    <!-- 快捷记事本弹窗组件 -->
    <QuickNotepad
      :visible="notepadVisible"
      @close="notepadVisible = false"
      @type="onTypeSnippet"
    />

    <!-- 轻量连接状态胶囊 (仅连接中展示) -->
    <div
      v-if="statusText"
      class="fixed top-4 left-1/2 -translate-x-1/2 z-40 px-3.5 py-1.5 rounded-full bg-zinc-900/90 border border-zinc-700/80 backdrop-blur text-xs font-medium text-zinc-300 flex items-center gap-2 shadow-xl"
    >
      <div v-if="connecting" class="w-2.5 h-2.5 rounded-full border-2 border-zinc-300 border-t-transparent animate-spin"></div>
      <div v-else class="w-2 h-2 rounded-full bg-zinc-300"></div>
      <span>{{ statusText }}</span>
      <button v-if="!connecting" @click="connectDesktop(true)" class="underline ml-2">重新连接</button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { api, ensureSandbox } from '../api.js'
import DraggableDock from '../components/DraggableDock.vue'
import QuickNotepad from '../components/QuickNotepad.vue'

const screenRef = ref(null)
const notepadVisible = ref(false)
const statusText = ref('正在连接图形远程桌面...')
const connecting = ref(true)

let rfb = null
let disposed = false
let controller = null
let RFB = null

async function connectDesktop(reuseOnly = false) {
  if (disposed || controller || rfb) return
  const attempt = new AbortController()
  controller = attempt
  connecting.value = true
  try {
    statusText.value = '正在载入桌面渲染核心...'
    if (!RFB) {
      const importEsm = new Function('u', 'return import(u)')
      RFB = (await importEsm('/vnc/core/rfb.js')).default
    }
    if (disposed || attempt.signal.aborted) return
    statusText.value = '正在获取远程桌面连接凭证...'
    const act = reuseOnly
      ? await api.getActiveSandbox('stream=gui', attempt.signal)
      : await ensureSandbox(attempt.signal)
    if (disposed || attempt.signal.aborted) return
    if (reuseOnly && !act.active) throw new Error('沙箱已关闭，请从工作区重新启动')
    const workerUrl = act.gui_worker_url
    const ticket = act.gui_ticket
    if (!workerUrl || !ticket) throw new Error('未获取到桌面连接凭证')
    const u = new URL(workerUrl)
    if (u.hostname === '127.0.0.1' || u.hostname === 'localhost') u.hostname = window.location.hostname
    u.searchParams.set('ticket', ticket)
    statusText.value = '正在连接 Worker 桌面服务...'
    const current = new RFB(screenRef.value, u.toString(), {
      credentials: {}, wsProtocols: ['binary']
    })
    rfb = current
    current.scaleViewport = true
    current.resizeSession = true
    current.clipViewport = false
    current.background = '#09090b'
    current.compressionLevel = 4
    current.qualityLevel = 7
    current.addEventListener('connect', () => {
      if (disposed || rfb !== current) return
      statusText.value = ''
      connecting.value = false
      current.focus()
    })
    current.addEventListener('disconnect', () => {
      if (disposed || rfb !== current) return
      rfb = null
      connecting.value = false
      statusText.value = '连接已断开，可以重新连接原桌面'
    })
    current.addEventListener('clipboard', (e) => {
      if (e.detail?.text && navigator.clipboard) {
        navigator.clipboard.writeText(e.detail.text).catch(() => {})
      }
    })
  } catch (err) {
    if (!disposed && !attempt.signal.aborted) {
      statusText.value = '桌面连接失败: ' + err.message
      connecting.value = false
    }
  } finally {
    if (controller === attempt) controller = null
  }
}

function onOpenRequest(e) {
  if (e.origin === window.location.origin && e.data?.type === 'attackbox:open-viewer') {
    return connectDesktop()
  }
}

onMounted(() => {
  window.addEventListener('message', onOpenRequest)
  connectDesktop()
})

onBeforeUnmount(() => {
  disposed = true
  window.removeEventListener('message', onOpenRequest)
  controller?.abort()
  const current = rfb
  rfb = null
  current?.disconnect()
})

// 键入文本到图形远程桌面
async function onTypeSnippet(text) {
  if (!text || !rfb) return
  rfb.focus()

  const target = rfb
  let i = 0
  for (const ch of text) {
    if (disposed || rfb !== target) return
    if (ch === '\r') continue
    let keysym = ch.codePointAt(0)
    if (keysym > 0xFF) keysym |= 0x01000000
    if (ch === '\n') keysym = 0xFF0D // Return
    else if (ch === '\t') keysym = 0xFF09 // Tab
    else if (ch === '\b') keysym = 0xFF08 // Backspace

    target.sendKey(keysym, null, true)
    target.sendKey(keysym, null, false)

    if (++i % 6 === 0) {
      await new Promise(r => setTimeout(r, 6))
    }
  }
}
</script>
