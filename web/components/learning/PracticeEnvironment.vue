<script setup lang="ts">
import { Power, Square, Terminal, Monitor } from 'lucide-vue-next'
import type { TargetDisplay } from '~/shared/learning-target'

const props = defineProps<{ context: Record<string, string>; target?: TargetDisplay }>()
const emit = defineEmits<{ open: [mode: 'cli' | 'gui'] }>()

const {
  sandbox,
  currentUser,
  busy,
  loading,
  notice,
  countdown: sandboxCountdown,
  start: startSandbox,
  release: stopSandbox
} = useSandboxSession()

const {
  current,
  initializing,
  action,
  requestPending,
  message,
  countdown: targetCountdown,
  config,
  unavailable,
  active,
  canRetry,
  status,
  run: runTarget
} = useTargetSession(props)

const instance = computed(() => sandbox.value.instance)
const ready = computed(() => instance.value?.state === 'running' && instance.value.connected)
const sandboxPending = computed(
  () =>
    busy.value ||
    loading.value ||
    ['creating', 'stopping'].includes(instance.value?.state || '')
)
const targetPending = computed(
  () => initializing.value || !!action.value || requestPending.value
)
const problem = computed(() => notice.value || message.value || instance.value?.error || '')

async function handleStartSandbox() {
  if (sandboxPending.value || !currentUser.value) return
  await startSandbox()
  if (ready.value) emit('open', 'cli')
}
</script>
<template>
  <LearningEnvironmentPanel :class="{ 'target-runtime': !!props.target }">
    <template #controls>
      <NuxtLink
        v-if="!currentUser"
        :to="{ path: '/auth', query: { returnTo: $route.fullPath } }"
        class="login-link"
        >登录后启动环境</NuxtLink
      >
      <template v-else>
        <UiButton
          v-if="!instance"
          size="sm"
          :loading="sandboxPending"
          :disabled="!sandbox.available"
          @click="handleStartSandbox"
          ><Power :size="14" />启动环境</UiButton
        >
        <UiButton
          v-else
          variant="danger"
          size="sm"
          :loading="sandboxPending"
          :aria-label="instance.state === 'cleanup_failed' ? '重试释放' : '停止环境'"
          @click="stopSandbox"
          ><Square :size="12" /><span>{{
            instance.state === 'cleanup_failed'
              ? '重试释放'
              : instance.connected
                ? '停止环境'
                : '节点断开 · 释放环境'
          }}</span
          ><time v-if="instance.state === 'running' && sandboxCountdown">{{
            sandboxCountdown
          }}</time></UiButton
        >
        <UiButton v-if="ready" variant="secondary" size="sm" @click="emit('open', 'cli')"
          ><Terminal :size="14" />CLI 终端</UiButton
        >
        <UiButton v-if="ready" variant="secondary" size="sm" @click="emit('open', 'gui')"
          ><Monitor :size="14" />GUI 桌面</UiButton
        >
        <template v-if="props.target">
          <span v-if="active && status" class="state" role="status">{{ status }}</span>
          <UiButton
            v-if="(!active || canRetry) && !unavailable"
            size="sm"
            :loading="targetPending"
            @click="runTarget('start')"
            ><Power :size="14" />启动靶机</UiButton
          >
          <UiButton
            v-if="active"
            variant="danger"
            size="sm"
            :loading="targetPending"
            @click="runTarget('stop')"
            ><Square :size="12" />停止靶机
            <time v-if="targetCountdown">{{ targetCountdown }}</time></UiButton
          >
          <span v-if="unavailable && !active" class="state">靶机暂不可用</span>
        </template>
      </template>
    </template>
    <LearningEnvironmentAddresses v-if="active" :instance="current?.instance" />
    <span v-if="active && !current?.instance?.entry_address && !current?.instance?.ip" class="state"
      >IP 待分配</span
    >
    <p v-if="config?.accessInstructions" class="instructions">{{ config.accessInstructions }}</p>
    <UiNotice v-if="problem" variant="error">{{ problem }}</UiNotice>
  </LearningEnvironmentPanel>
</template>
<style scoped>
time {
  font-variant-numeric: tabular-nums;
}
.login-link {
  font-size: 13px;
  color: var(--green);
  text-decoration: none;
}
.state {
  font-size: 12px;
  color: var(--muted);
}
.instructions {
  margin: 12px 0 0;
  white-space: pre-wrap;
  color: var(--muted);
  font-size: 13px;
}
</style>
