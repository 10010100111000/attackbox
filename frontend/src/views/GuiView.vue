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
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { api } from '../api.js'
import DraggableDock from '../components/DraggableDock.vue'
import QuickNotepad from '../components/QuickNotepad.vue'

const screenRef = ref(null)
const notepadVisible = ref(false)
const statusText = ref('正在连接图形远程桌面...')
const connecting = ref(true)

let rfb = null

onMounted(async () => {
  try {
    statusText.value = '正在获取远程桌面连接凭证...'
    let act = await api.getActiveSandbox('stream=gui').catch(() => null)
    if (!act || !act.active) {
      statusText.value = '正在按需拉起沙箱环境...'
      act = await api.startSandbox().catch((err) => {
        throw new Error('启动沙箱环境失败: ' + (err.message || err))
      })
    }

    const workerUrl = act.gui_worker_url || act.worker_url
    const ticket = act.gui_ticket || act.ticket
    if (!workerUrl || !ticket) {
      throw new Error('未获取到 Worker 节点数据通道或 Ed25519 Ticket')
    }

    let wsUrl = workerUrl
    try {
      const u = new URL(workerUrl)
      if (!u.pathname.endsWith('/ws/gui')) {
        u.pathname = u.pathname.replace(/\/ws\/cli$/, '') + '/ws/gui'
      }
      u.searchParams.set('ticket', ticket)
      wsUrl = u.toString()
    } catch (_) {
      const sep = workerUrl.includes('?') ? '&' : '?'
      wsUrl = `${workerUrl}${sep}ticket=${ticket}`
    }

    statusText.value = '正在载入桌面渲染核心...'
    // 动态载入后端静态挂载的 noVNC RFB 核心 (使用原生动态导入)
    const importEsm = new Function('u', 'return import(u)')
    const { default: RFB } = await importEsm('/vnc/core/rfb.js')

    statusText.value = '正在连接 Worker 桌面服务...'
    rfb = new RFB(screenRef.value, wsUrl, {
      credentials: {},
      wsProtocols: ['binary']
    })

    rfb.scaleViewport = true
    rfb.resizeSession = true
    rfb.clipViewport = false
    rfb.background = '#09090b'

    rfb.addEventListener('connect', () => {
      statusText.value = ''
      connecting.value = false
      rfb.focus()
    })

    rfb.addEventListener('disconnect', (e) => {
      connecting.value = false
      if (e.detail && e.detail.clean) {
        statusText.value = '远程桌面会话已关闭'
      } else {
        statusText.value = '连接中断，请刷新重试'
      }
    })

    // 无感远程剪贴板监听
    rfb.addEventListener('clipboard', (e) => {
      if (e.detail && e.detail.text) {
        try {
          navigator.clipboard.writeText(e.detail.text)
        } catch (_) {}
      }
    })
  } catch (err) {
    statusText.value = '加载远程桌面组件失败: ' + err.message
    connecting.value = false
  }
})

onBeforeUnmount(() => {
  if (rfb) {
    try { rfb.disconnect() } catch (_) {}
  }
})

// 键入文本到图形远程桌面
async function onTypeSnippet(text) {
  if (!text || !rfb) return
  rfb.focus()

  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (ch === '\r') continue
    let keysym = ch.charCodeAt(0)
    if (ch === '\n') keysym = 0xFF0D // Return
    else if (ch === '\t') keysym = 0xFF09 // Tab
    else if (ch === '\b') keysym = 0xFF08 // Backspace

    rfb.sendKey(keysym, null, true)
    rfb.sendKey(keysym, null, false)

    if (i % 6 === 0) {
      await new Promise(r => setTimeout(r, 6))
    }
  }
}
</script>
