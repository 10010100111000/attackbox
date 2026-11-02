<script setup lang="ts">
import { Power, Square, Terminal, Monitor } from 'lucide-vue-next'
import type { TargetDisplay } from '~/shared/learning-target'
const props = defineProps<{ context: Record<string, string>; target?: TargetDisplay }>()
const emit = defineEmits<{ open: [mode: 'cli' | 'gui'] }>()
const session = useSandboxSession()
const { sandbox, currentUser, busy, loading, notice } = session
const target = useTargetSession(props, { followSandbox: false })
const { current, message, config, unavailable, active } = target
const confirm = useConfirm()
const operating = ref(false)
let alive = true
onBeforeUnmount(() => {
  alive = false
})
const instance = computed(() => sandbox.value.instance)
const ready = computed(() => instance.value?.state === 'running' && instance.value.connected)
const pending = computed(
  () =>
    operating.value ||
    busy.value ||
    loading.value ||
    target.initializing.value ||
    target.requestPending.value ||
    !!target.action.value ||
    ['creating', 'stopping'].includes(instance.value?.state || '')
)
const hasResources = computed(() => !!instance.value || active.value)
const complete = computed(
  () =>
    ready.value &&
    (!props.target ||
      (target.state.value === 'running' &&
        current.value?.instance?.ready &&
        target.remaining.value !== 0))
)
const canStart = computed(
  () =>
    !complete.value &&
    (!instance.value || ready.value) &&
    (!active.value ||
      (target.remaining.value !== 0 &&
        (target.canRetry.value ||
          (target.state.value === 'running' && current.value?.instance?.ready))))
)
const problem = computed(
  () =>
    notice.value ||
    message.value ||
    instance.value?.error ||
    (active.value &&
    (target.remaining.value === 0 ||
      [
        'failed',
        'incomplete',
        'degraded',
        'missing',
        'orphaned',
        'unconfirmed',
        'cleanup_pending'
      ].includes(target.state.value))
      ? target.status.value
      : '')
)
const remaining = computed(() => {
  const values = [session.seconds.value, active.value ? target.remaining.value : null].filter(
    (value): value is number => value !== null
  )
  if (!values.length) return ''
  const seconds = Math.min(...values)
  return `${Math.floor(seconds / 60)
    .toString()
    .padStart(2, '0')}:${(seconds % 60).toString().padStart(2, '0')}`
})
async function start() {
  if (pending.value || unavailable.value || !currentUser.value) return
  operating.value = true
  try {
    // Allocate the target first so the existing backend places the attack
    // sandbox on a node that actually has this prepared teaching environment.
    if (props.target && (!active.value || target.canRetry.value)) await target.run('start')
    if (
      !alive ||
      (props.target && (target.state.value !== 'running' || !current.value?.instance?.ready))
    )
      return
    const startingSandbox = !ready.value
    if (startingSandbox) await session.start()
    if (!alive || !ready.value) return
    // One action-triggered read retrieves the new attachment / reverse IP.
    if (props.target && startingSandbox) await target.run('inspect')
    if (alive && complete.value) emit('open', 'cli')
  } finally {
    operating.value = false
  }
}
async function stop() {
  if (pending.value || !hasResources.value) return
  const actor = currentUser.value?.id,
    sandboxId = instance.value?.id,
    targetId = current.value?.id
  if (
    !(await confirm({
      title: '释放实验环境？',
      description: '将释放本章靶机及沙箱，关闭终端和桌面，并清除其中的修改。',
      confirmLabel: '释放环境'
    }))
  )
    return
  if (
    pending.value ||
    currentUser.value?.id !== actor ||
    instance.value?.id !== sandboxId ||
    current.value?.id !== targetId
  )
    return
  operating.value = true
  try {
    if (active.value) await target.run('stop', false)
    if (active.value) return
    if (!alive || currentUser.value?.id !== actor || instance.value?.id !== sandboxId) return
    await session.releaseConfirmed()
  } finally {
    operating.value = false
  }
}
</script>
<template>
  <LearningEnvironmentPanel :class="{ 'target-runtime': !!props.target }">
    <template #controls>
      <NuxtLink v-if="!currentUser" :to="{ path: '/auth', query: { returnTo: $route.fullPath } }"
        >登录后启动环境</NuxtLink
      >
      <template v-else>
        <UiButton
          v-if="canStart"
          size="sm"
          :loading="pending"
          :disabled="unavailable || (!instance && !sandbox.available)"
          @click="start"
          ><Power :size="14" />启动环境</UiButton
        >
        <UiButton
          v-if="hasResources"
          variant="danger"
          size="sm"
          :loading="pending"
          aria-label="释放环境"
          @click="stop"
          ><Square :size="12" /><time v-if="remaining">{{ remaining }}</time
          ><span v-else>释放环境</span></UiButton
        >
        <UiButton v-if="ready" variant="secondary" size="sm" @click="emit('open', 'cli')"
          ><Terminal :size="14" />CLI 终端</UiButton
        >
        <UiButton v-if="ready" variant="secondary" size="sm" @click="emit('open', 'gui')"
          ><Monitor :size="14" />GUI 桌面</UiButton
        >
        <span v-if="unavailable" class="state">靶机暂不可用</span>
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
