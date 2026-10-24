<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { Copy, Check } from 'lucide-vue-next'

const props = defineProps<{
  context: Record<string, string>
}>()

const current = ref<any>(null)
const busy = ref(false)
const message = ref('')
const mounted = ref(false)
const copied = ref(false)
let timer: ReturnType<typeof setInterval> | undefined

const stateLabels: Record<string, string> = {
  starting: '正在启动',
  created: '已创建',
  running: '运行中',
  stopping: '正在停止',
  stopped: '已停止',
  cleanup_pending: '清理待确认',
  missing: '实例缺失',
  orphaned: '待接管',
}

const readinessLabels: Record<string, string> = {
  healthy: '服务健康',
  ports_open: '端口已开放',
  waiting_for_ports: '等待服务端口就绪…',
  unverified: '已运行（无健康探针）',
  network_unavailable: '网络未就绪',
  not_running: '未运行',
}

const statusText = computed(() => {
  if (!current.value) return '未启动'
  if (current.value.instance?.ready) return '服务已就绪'
  const state = current.value.state || ''
  const readiness = current.value.instance?.readiness
  if (state === 'running' && readiness && readinessLabels[readiness]) {
    return readinessLabels[readiness]
  }
  return stateLabels[state] || state
})

async function copyIp() {
  const ip = current.value?.instance?.ip
  if (!ip || !navigator?.clipboard) return
  try {
    await navigator.clipboard.writeText(ip)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {}
}

async function run(action: 'inspect' | 'start' | 'stop') {
  if (busy.value) return
  busy.value = true
  message.value = ''
  try {
    const result =
      action === 'inspect'
        ? await $fetch('/api/targets', { query: props.context })
        : await $fetch('/api/targets', {
            method: 'POST',
            body: { ...props.context, action, id: current.value?.id },
          })
    current.value = result
    message.value = (result as any)?.error || ''
  } catch (e: any) {
    message.value = e?.data?.message || '环境操作失败，请稍后重试'
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  mounted.value = true
  run('inspect')
  timer = setInterval(() => {
    if (!busy.value && current.value && current.value.state !== 'stopped') {
      run('inspect')
    }
  }, 8000)
})

onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <section class="target-runtime" aria-label="靶机运行状态">
    <div class="target-header">
      <strong>实验靶机</strong>
      <span
        v-if="current"
        class="target-badge"
        :class="{
          ready: current.instance?.ready,
          running: current.state === 'running' && !current.instance?.ready,
          stopped: current.state === 'stopped',
        }"
      >
        {{ statusText }}
      </span>
    </div>

    <div v-if="current?.instance?.ip" class="target-ip-row">
      <div class="ip-box">
        <span class="ip-label">目标地址</span>
        <code>{{ current.instance.ip }}</code>
        <button
          type="button"
          class="copy-btn"
          :aria-label="copied ? '已复制目标地址' : '复制目标地址'"
          @click="copyIp"
        >
          <Check v-if="copied" :size="13" aria-hidden="true" />
          <Copy v-else :size="13" aria-hidden="true" />
          <span>{{ copied ? '已复制' : '复制' }}</span>
        </button>
      </div>
      <small>使用同节点 CLI / GUI 访问</small>
    </div>

    <p v-if="message" class="target-message" role="status">
      {{ message }}
    </p>

    <footer class="target-actions">
      <button
        type="button"
        class="primary-btn"
        :disabled="!mounted || busy || current?.state === 'stopping'"
        @click="run('start')"
      >
        {{
          busy
            ? '处理中…'
            : current && current.state !== 'stopped'
              ? '继续启动／重试'
              : '启动靶机'
        }}
      </button>
      <button
        v-if="current && current.state !== 'stopped'"
        type="button"
        class="danger-btn"
        :disabled="busy"
        @click="run('stop')"
      >
        停止靶机
      </button>
      <button
        type="button"
        class="secondary-btn"
        :disabled="!mounted || busy"
        @click="run('inspect')"
      >
        刷新状态
      </button>
    </footer>
  </section>
</template>

<style scoped>
.target-runtime {
  margin: 18px 0;
  padding: 18px 20px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--paper);
  font-size: 13px;
}

.target-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.target-badge {
  display: inline-flex;
  align-items: center;
  padding: 3px 9px;
  border-radius: 5px;
  font-size: 11px;
  font-weight: 550;
  background: var(--soft);
  color: var(--muted);
}

.target-badge.ready {
  background: var(--success-bg);
  color: var(--success-ink);
}

.target-badge.running {
  background: var(--mint);
  color: var(--green);
}

.target-ip-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 14px;
  padding: 10px 14px;
  border-radius: 6px;
  background: var(--soft);
}

.ip-box {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.ip-label {
  font-size: 11px;
  color: var(--muted);
}

.ip-box code {
  font: 600 13px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  color: var(--ink);
}

.copy-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  border: 1px solid var(--line);
  border-radius: 4px;
  background: var(--paper);
  color: var(--muted);
  font-size: 11px;
  cursor: pointer;
}

.copy-btn:hover {
  color: var(--ink);
}

.target-ip-row small {
  color: var(--muted);
  font-size: 12px;
}

.target-message {
  margin: 12px 0 0;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--error-bg);
  color: var(--error-ink);
  font-size: 12px;
  line-height: 1.6;
}

.target-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 16px;
}

button {
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 8px 14px;
  background: var(--paper);
  color: var(--ink);
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
  transition: opacity 0.15s ease, background 0.15s ease;
}

.primary-btn {
  background: var(--ink);
  color: var(--paper);
  border-color: var(--ink);
}

.danger-btn:hover:not(:disabled) {
  background: var(--error-bg);
  color: var(--error-ink);
  border-color: transparent;
}

.secondary-btn:hover:not(:disabled) {
  background: var(--soft);
}

button:disabled {
  opacity: 0.5;
  cursor: default;
}
</style>
