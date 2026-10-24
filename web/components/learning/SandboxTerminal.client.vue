<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import type { Terminal } from '@xterm/xterm'
import type { FitAddon } from '@xterm/addon-fit'
import type { SandboxTicket } from '~/shared/sandbox'
import '@xterm/xterm/css/xterm.css'
const props = defineProps<{ sandboxId: string; windowId: string; active: boolean }>()
const host = ref<HTMLElement | null>(null), state = ref('正在连接'), reconnecting = ref(false)
let terminal: Terminal | undefined, fit: FitAddon | undefined, socket: WebSocket | undefined
let resize: ResizeObserver | undefined, retryTimer: ReturnType<typeof setTimeout> | undefined
let stableTimer: ReturnType<typeof setTimeout> | undefined, connectTimer: ReturnType<typeof setTimeout> | undefined
let destroyed = false, generation = 0, retries = 0, hadConnection = false
function fitTerminal() {
  if (!props.active || !host.value?.clientWidth || !host.value.clientHeight || !terminal || !fit) return
  fit.fit()
  if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ cols: terminal.cols, rows: terminal.rows }))
}
async function connect() {
  if (destroyed || reconnecting.value) return
  clearTimeout(retryTimer)
  const ownGeneration = ++generation
  reconnecting.value = true; state.value = '正在连接'
  const old = socket; socket = undefined; old?.close()
  try {
    const response = await $fetch<{ data: SandboxTicket }>('/api/sandbox/ticket', { method: 'POST', body: { sandboxId: props.sandboxId } })
    if (destroyed || ownGeneration !== generation) return
    const url = new URL(response.data.url)
    // The controller issues a loopback URL. The browser must dial the page host
    // instead, while keeping the worker's own port (8443) and scheme.
    if (typeof window !== 'undefined' && (url.hostname === '127.0.0.1' || url.hostname === 'localhost')) {
      url.hostname = window.location.hostname
    }
    url.searchParams.set('ticket', response.data.ticket); url.searchParams.set('win', props.windowId)
    const connection = new WebSocket(url); socket = connection; connection.binaryType = 'arraybuffer'
    connectTimer = setTimeout(() => { if (socket === connection && connection.readyState === WebSocket.CONNECTING) connection.close() }, 10000)
    connection.onopen = () => {
      if (destroyed || socket !== connection) { connection.close(); return }
      if (hadConnection) terminal?.writeln('\r\n\x1b[33m连接已恢复，当前为新 Shell；沙箱文件仍保留。\x1b[0m')
      clearTimeout(connectTimer); clearTimeout(stableTimer)
      stableTimer = setTimeout(() => { retries = 0 }, 15000)
      hadConnection = true; state.value = '已连接'; reconnecting.value = false
      fitTerminal(); if (props.active) terminal?.focus()
    }
    connection.onmessage = event => {
      if (socket !== connection || destroyed) return
      if (event.data instanceof ArrayBuffer) terminal?.write(new Uint8Array(event.data))
      else terminal?.write(String(event.data))
    }
    connection.onclose = event => {
      if (socket !== connection || destroyed) return
      clearTimeout(connectTimer); clearTimeout(stableTimer)
      reconnecting.value = false
      if (event.code === 4001) { state.value = '此终端已被另一连接接管'; return }
      state.value = '连接已断开'
      scheduleRetry()
    }
    connection.onerror = () => { state.value = '终端连接失败' }
  } catch (error: any) {
    if (!destroyed && ownGeneration === generation) { reconnecting.value = false; state.value = error?.data?.message || '无法连接终端'; scheduleRetry() }
  }
}
function scheduleRetry() {
  if (destroyed || retries >= 3) return
  retryTimer = setTimeout(connect, 1000 * 2 ** retries++)
}
watch(() => props.active, async active => { if (active) { await nextTick(); fitTerminal() } })
onMounted(async () => {
  const [{ Terminal: Xterm }, { FitAddon: Fit }] = await Promise.all([import('@xterm/xterm'), import('@xterm/addon-fit')])
  if (destroyed || !host.value) return
  terminal = new Xterm({ cursorBlink: true, fontSize: 14, fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace', scrollback: 3000, theme: { background: '#101419', foreground: '#dce4eb', cursor: '#8ed6bd', selectionBackground: '#3b5364' } })
  fit = new Fit(); terminal.loadAddon(fit); terminal.open(host.value)
  terminal.onData(value => {
    if (socket?.readyState !== WebSocket.OPEN) return
    const bytes = new TextEncoder().encode(value)
    for (let offset = 0; offset < bytes.length; offset += 16384) socket.send(bytes.slice(offset, offset + 16384))
  })
  resize = new ResizeObserver(fitTerminal); resize.observe(host.value)
  await nextTick(); fitTerminal(); await connect()
})
onBeforeUnmount(() => { destroyed = true; generation++; clearTimeout(retryTimer); clearTimeout(stableTimer); clearTimeout(connectTimer); resize?.disconnect(); socket?.close(); terminal?.dispose() })
</script>

<template>
  <div class="terminal-panel"><div ref="host" class="terminal-host" aria-label="交互终端" /><div class="terminal-status" role="status"><span>{{ state }}</span><button v-if="state !== '已连接'" type="button" :disabled="reconnecting" @click="retries = 0; connect()">重新连接</button></div></div>
</template>

<style scoped>
.terminal-panel { height: 100%; min-height: 0; display: flex; flex-direction: column; background: #101419; color: #a9b7c3; }
.terminal-host { flex: 1; min-height: 0; padding: 12px 14px 4px; overflow: hidden; }
.terminal-host :deep(.xterm) { height: 100%; }
.terminal-status { flex-shrink: 0; min-height: 25px; padding: 3px 14px; border-top: 1px solid #26313b; font-size: 11px; display: flex; align-items: center; gap: 14px; }
.terminal-status button { color: #8ed6bd; background: transparent; border: 0; cursor: pointer; font-size: 11px; }
.terminal-status button:disabled { opacity: .5; cursor: wait; }
</style>
