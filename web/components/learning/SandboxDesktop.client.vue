<script setup lang="ts">
import { Clipboard } from 'lucide-vue-next'
import { PopoverRoot, PopoverTrigger, PopoverPortal, PopoverContent, PopoverArrow } from 'reka-ui'
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useSandboxSession } from '~/composables/useSandboxSession'
import type RFB from '@novnc/novnc'
import type { SandboxTicket } from '~/shared/sandbox'
import { apiErrorMessage, apiFetch } from '~/utils/api'
import {
  onClipboardSync,
  readBrowserClipboard,
  writeBrowserClipboard
} from '~/utils/clipboard'
const props = defineProps<{ sandboxId: string; active: boolean; autoResize?: boolean }>()
const { announceGUI } = useSandboxSession()
const host = ref<HTMLElement | null>(null),
  state = ref(''),
  connecting = ref(false),
  fit = ref(true)
const clipboardOpen = ref(false),
  clipboardText = ref(''),
  clipboardMessage = ref('')
let viewer: RFB | undefined,
  retryTimer: ReturnType<typeof setTimeout> | undefined,
  connectTimer: ReturnType<typeof setTimeout> | undefined,
  stableTimer: ReturnType<typeof setTimeout> | undefined,
  stopClipboardSync: (() => void) | undefined
let disposed = false,
  generation = 0,
  retries = 0,
  lastSyncedClipboard = ''

function pushClipboardToDesktop(rawText: string) {
  if (disposed || !viewer || state.value !== '桌面已连接') return false
  const text = rawText.slice(0, 65536)
  if (!text) return false
  lastSyncedClipboard = text
  clipboardText.value = text
  viewer.clipboardPasteFrom(text)
  return true
}

async function syncBrowserToDesktop() {
  if (disposed || !viewer || state.value !== '桌面已连接' || clipboardOpen.value) return
  if (typeof navigator === 'undefined' || !navigator.clipboard?.readText) return
  try {
    const text = (await navigator.clipboard.readText()).slice(0, 65536)
    if (!text || text === lastSyncedClipboard) return
    pushClipboardToDesktop(text)
  } catch {
    /* Silent when browser blocks background clipboard reads; manual popover or paste event still works. */
  }
}

function onDesktopPaste(event: ClipboardEvent) {
  const text = event.clipboardData?.getData('text/plain') || ''
  if (!text) return
  if (pushClipboardToDesktop(text)) {
    clipboardMessage.value = '已发送到桌面，可在桌面内粘贴。'
  }
}

function onDesktopCopy(event: ClipboardEvent) {
  if (!clipboardText.value) return
  event.preventDefault()
  event.clipboardData?.setData('text/plain', clipboardText.value)
}

function reconnect() {
  retries = 0
  connect()
}
async function connect() {
  if (disposed || connecting.value || !host.value) return
  const ownGeneration = ++generation
  clearTimeout(retryTimer)
  clearTimeout(connectTimer)
  clearTimeout(stableTimer)
  connecting.value = true
  state.value = ''
  const old = viewer
  viewer = undefined
  old?.disconnect()
  try {
    const [{ default: Desktop }, response] = await Promise.all([
      import('@novnc/novnc'),
      apiFetch<{ data: SandboxTicket }>('/api/sandbox/ticket', {
        method: 'POST',
        body: { sandboxId: props.sandboxId, stream: 'gui' }
      })
    ])
    if (disposed || generation !== ownGeneration || !host.value) return
    const url = new URL(response.data.url)
    url.searchParams.set('ticket', response.data.ticket)
    const connection = new Desktop(host.value, url.toString(), { shared: true })
    viewer = connection
    connection.scaleViewport = fit.value
    connection.resizeSession = !!props.autoResize && props.active && fit.value
    connection.clipViewport = false
    connection.compressionLevel = 4
    connection.qualityLevel = 7
    connectTimer = setTimeout(() => {
      if (viewer === connection) {
        state.value = '桌面连接超时'
        connection.disconnect()
      }
    }, 35000)
    connection.addEventListener('connect', () => {
      if (disposed || viewer !== connection) {
        connection.disconnect()
        return
      }
      clearTimeout(connectTimer)
      connecting.value = false
      state.value = '桌面已连接'
      announceGUI()
      stableTimer = setTimeout(() => {
        retries = 0
      }, 15000)
      if (props.active) connection.focus()
    })
    connection.addEventListener('disconnect', () => {
      if (disposed || viewer !== connection) return
      clearTimeout(connectTimer)
      clearTimeout(stableTimer)
      connecting.value = false
      state.value = '桌面连接已断开'
      // Explicit retry avoids reclaiming a desktop currently used by another browser.
    })
    connection.addEventListener('clipboard', event => {
      if (disposed || viewer !== connection) return
      const text = (event as CustomEvent<{ text: string }>).detail.text.slice(0, 65536)
      lastSyncedClipboard = text
      clipboardText.value = text
      clipboardMessage.value = '已接收桌面剪贴板'
      void writeBrowserClipboard(text, { broadcast: false })
    })
    connection.addEventListener('securityfailure', () => {
      if (viewer === connection) state.value = '桌面认证失败，请检查节点配置'
    })
  } catch (error) {
    if (!disposed && generation === ownGeneration) {
      connecting.value = false
      state.value = apiErrorMessage(error, '无法连接桌面')
      if (retries < 3) retryTimer = setTimeout(connect, 1000 * 2 ** retries++)
    }
  }
}
async function readClipboard() {
  const text = await readBrowserClipboard()
  if (text !== null) {
    clipboardText.value = text.slice(0, 65536)
    clipboardMessage.value = ''
  } else {
    clipboardMessage.value = '请在文本框中使用 Ctrl+V 粘贴。'
  }
}
async function copyClipboard() {
  if (await writeBrowserClipboard(clipboardText.value, { broadcast: false })) {
    clipboardMessage.value = '已复制到浏览器剪贴板'
  } else {
    clipboardMessage.value = '请选中文本，使用 Ctrl+C 复制。'
  }
}
function sendClipboard() {
  if (!viewer || state.value !== '桌面已连接') return
  lastSyncedClipboard = clipboardText.value.slice(0, 65536)
  viewer.clipboardPasteFrom(lastSyncedClipboard)
  clipboardMessage.value = '已发送到桌面，可在桌面内粘贴。'
}
watch(fit, value => {
  if (viewer) {
    viewer.scaleViewport = value
    viewer.resizeSession = !!props.autoResize && props.active && value
  }
})
watch(
  () => props.active,
  async active => {
    await nextTick()
    if (viewer) viewer.resizeSession = !!props.autoResize && active && fit.value
    if (active) viewer?.focus()
  }
)
onMounted(() => {
  stopClipboardSync = onClipboardSync(text => {
    if (disposed || !viewer || state.value !== '桌面已连接' || text === lastSyncedClipboard) return
    pushClipboardToDesktop(text)
  })
  window.addEventListener('focus', syncBrowserToDesktop)
  void connect()
})
onBeforeUnmount(() => {
  disposed = true
  generation++
  stopClipboardSync?.()
  window.removeEventListener('focus', syncBrowserToDesktop)
  clearTimeout(retryTimer)
  clearTimeout(connectTimer)
  clearTimeout(stableTimer)
  const old = viewer
  viewer = undefined
  old?.disconnect()
})
</script>

<template>
  <div class="desktop-panel">
    <div class="desktop-toolbar">
      <span v-if="state" role="status">{{ state }}</span
      ><UiBusyIndicator v-else inline label="桌面连接" /><label
        ><UiCheckbox v-model="fit" />自适应（取消后为原始尺寸）</label
      ><PopoverRoot v-model:open="clipboardOpen"
        ><PopoverTrigger
          class="desktop-action clipboard-trigger"
          :disabled="state !== '桌面已连接'"
          aria-label="桌面剪贴板"
          ><Clipboard :size="13" aria-hidden="true" />剪贴板</PopoverTrigger
        ><PopoverPortal
          ><PopoverContent
            class="desktop-clipboard"
            side="bottom"
            align="end"
            :side-offset="8"
            aria-label="桌面剪贴板"
          >
            <label
              >剪贴板<UiTextarea
                v-model="clipboardText"
                aria-label="剪贴板文本"
                rows="5"
                maxlength="65536"
                placeholder="在此粘贴，或在远程桌面中复制文本。"
              />
            </label>
            <div class="clipboard-actions">
              <button type="button" @click="readClipboard">读取浏览器</button
              ><button type="button" @click="copyClipboard">复制到浏览器</button
              ><button
                type="button"
                class="send"
                :disabled="state !== '桌面已连接'"
                @click="sendClipboard"
              >
                发送到桌面
              </button>
            </div>
            <p v-if="clipboardMessage" role="status">{{ clipboardMessage }}</p>
            <PopoverArrow class="clipboard-arrow" /> </PopoverContent></PopoverPortal></PopoverRoot
      ><button
        v-if="state === '桌面已连接'"
        class="desktop-action"
        type="button"
        @click="viewer?.sendCtrlAltDel()"
      >
        Ctrl Alt Del</button
      ><button
        v-else-if="state"
        class="desktop-action"
        type="button"
        :disabled="connecting"
        @click="reconnect"
      >
        重新连接
      </button>
    </div>
    <div
      ref="host"
      class="desktop-host"
      aria-label="远程桌面"
      @pointerenter="syncBrowserToDesktop"
      @pointerdown="syncBrowserToDesktop"
      @focusin="syncBrowserToDesktop"
      @paste="onDesktopPaste"
      @copy="onDesktopCopy"
    />
  </div>
</template>

<style scoped>
.desktop-panel {
  min-width: 0;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: #101419;
  color: #b5c1cc;
}
.desktop-toolbar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px 14px;
  flex-wrap: wrap;
  padding: 6px 14px;
  min-height: 30px;
  font-size: 11px;
  border-bottom: 1px solid #29343d;
}
.desktop-toolbar > span {
  margin-right: auto;
}
.desktop-toolbar label {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.desktop-action {
  background: transparent;
  border: 0;
  color: #7dd3fc;
  font-size: 11px;
  cursor: pointer;
}
.clipboard-trigger {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.desktop-action:disabled {
  opacity: 0.5;
  cursor: wait;
}
.desktop-host {
  min-width: 0;
  flex: 1;
  min-height: 0;
  overflow: auto;
}
</style>

<style>
.desktop-clipboard {
  z-index: 150;
  width: min(380px, calc(100vw - 32px));
  padding: 18px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--paper);
  color: var(--ink);
  box-shadow: 0 12px 40px #0003;
}
.desktop-clipboard label {
  display: grid;
  gap: 10px;
  font-size: 12px;
  font-weight: 600;
}
.desktop-clipboard textarea {
  width: 100%;
  padding: 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--soft);
  color: var(--ink);
  resize: vertical;
  font:
    12px/1.7 ui-monospace,
    monospace;
}
.clipboard-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}
.clipboard-actions button {
  padding: 7px 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  color: var(--ink);
  background: transparent;
  font-size: 11px;
  cursor: pointer;
}
.clipboard-actions .send {
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  border-color: transparent;
}
.desktop-clipboard p {
  margin: 12px 0 0;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.6;
}
.desktop-clipboard :focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 2px;
}
.clipboard-arrow {
  fill: var(--paper);
}
</style>
