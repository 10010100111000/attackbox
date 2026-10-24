<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import type RFB from '@novnc/novnc'
import type { SandboxTicket } from '~/shared/sandbox'
const props = defineProps<{ sandboxId: string; active: boolean }>()
const host = ref<HTMLElement | null>(null), state = ref('正在启动桌面'), connecting = ref(false), fit = ref(true)
let viewer: RFB | undefined, retryTimer: ReturnType<typeof setTimeout> | undefined, connectTimer: ReturnType<typeof setTimeout> | undefined, stableTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false, generation = 0, retries = 0
async function connect() {
  if (disposed || connecting.value || !host.value) return
  const ownGeneration = ++generation
  clearTimeout(retryTimer); clearTimeout(connectTimer); clearTimeout(stableTimer)
  connecting.value = true; state.value = '正在启动桌面…'
  const old = viewer; viewer = undefined; old?.disconnect()
  try {
    const [{ default: Desktop }, response] = await Promise.all([
      import('@novnc/novnc'),
      $fetch<{ data: SandboxTicket }>('/api/sandbox/ticket', { method: 'POST', body: { sandboxId: props.sandboxId, stream: 'gui' } })
    ])
    if (disposed || generation !== ownGeneration || !host.value) return
    const url = new URL(response.data.url)
    // Same loopback rewrite as the CLI: keep the worker port, only swap the host.
    if (typeof window !== 'undefined' && (url.hostname === '127.0.0.1' || url.hostname === 'localhost')) {
      url.hostname = window.location.hostname
    }
    url.searchParams.set('ticket', response.data.ticket)
    const connection = new Desktop(host.value, url.toString(), { shared: true }); viewer = connection
    connection.scaleViewport = fit.value; connection.resizeSession = false
    connectTimer = setTimeout(() => { if (viewer === connection) { state.value = '桌面连接超时'; connection.disconnect() } }, 35000)
    connection.addEventListener('connect', () => {
      if (disposed || viewer !== connection) { connection.disconnect(); return }
      clearTimeout(connectTimer); connecting.value = false; state.value = '桌面已连接'
      stableTimer = setTimeout(() => { retries = 0 }, 15000)
      if (props.active) connection.focus()
    })
    connection.addEventListener('disconnect', () => {
      if (disposed || viewer !== connection) return
      clearTimeout(connectTimer); clearTimeout(stableTimer); connecting.value = false; state.value = '桌面连接已断开'
      // Explicit retry avoids reclaiming a desktop currently used by another browser.
    })
    connection.addEventListener('securityfailure', () => { if (viewer === connection) state.value = '桌面认证失败，请检查节点配置' })
  } catch (error: any) {
    if (!disposed && generation === ownGeneration) {
      connecting.value = false; state.value = error?.data?.message || '无法连接桌面'
      if (retries < 3) retryTimer = setTimeout(connect, 1000 * 2 ** retries++)
    }
  }
}
watch(fit, value => { if (viewer) viewer.scaleViewport = value })
watch(() => props.active, async active => { if (active) { await nextTick(); viewer?.focus() } })
onMounted(connect)
onBeforeUnmount(() => { disposed = true; generation++; clearTimeout(retryTimer); clearTimeout(connectTimer); clearTimeout(stableTimer); const old = viewer; viewer = undefined; old?.disconnect() })
</script>

<template>
  <div class="desktop-panel"><div class="desktop-toolbar"><span role="status">{{ state }}</span><label><input v-model="fit" type="checkbox" />自适应</label><button v-if="state === '桌面已连接'" type="button" @click="viewer?.sendCtrlAltDel()">Ctrl Alt Del</button><button v-else type="button" :disabled="connecting" @click="retries = 0; connect()">重新连接</button></div><div ref="host" class="desktop-host" aria-label="远程桌面" /></div>
</template>

<style scoped>
.desktop-panel { display: flex; flex-direction: column; height: 100%; min-height: 0; background: #101419; color: #b5c1cc; }
.desktop-toolbar { flex-shrink: 0; display: flex; align-items: center; gap: 14px; padding: 6px 14px; min-height: 30px; font-size: 11px; border-bottom: 1px solid #29343d; }
.desktop-toolbar > span { margin-right: auto; }
.desktop-toolbar label { display: inline-flex; align-items: center; gap: 5px; }
.desktop-toolbar input { accent-color: #8ed6bd; }
.desktop-toolbar button { background: transparent; border: 0; color: #8ed6bd; font-size: 11px; cursor: pointer; }
.desktop-toolbar button:disabled { opacity: .5; cursor: wait; }
.desktop-host { flex: 1; min-height: 0; overflow: auto; }
</style>
