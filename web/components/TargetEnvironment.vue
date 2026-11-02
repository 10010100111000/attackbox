<script setup lang="ts">
import { Power, Square } from 'lucide-vue-next'
import type { TargetDisplay } from '~/shared/learning-target'
const props = defineProps<{ context: Record<string, string>; target?: TargetDisplay }>()
const { currentUser } = useAuth()
const {
  current,
  initializing,
  action,
  requestPending,
  message,
  countdown,
  config,
  unavailable,
  active,
  canRetry,
  status,
  run
} = useTargetSession(props)
const pending = computed(() => initializing.value || !!action.value || requestPending.value)
</script>
<template>
  <LearningEnvironmentPanel class="target-runtime">
    <template #controls>
      <span v-if="active" class="target-state" role="status">{{ status }}</span>
      <NuxtLink v-if="!currentUser" to="/auth">登录后启动</NuxtLink>
      <template v-else>
        <UiButton
          v-if="(!active || canRetry) && !unavailable"
          size="sm"
          :loading="pending"
          @click="run('start')"
          ><Power :size="14" />启动靶机</UiButton
        >
        <UiButton v-if="active" variant="danger" size="sm" :loading="pending" @click="run('stop')"
          ><Square :size="12" />停止靶机 <time v-if="countdown">{{ countdown }}</time></UiButton
        >
        <span v-if="unavailable && !active" class="target-state">暂不可用</span>
      </template>
    </template>
    <LearningEnvironmentAddresses v-if="active" :instance="current?.instance" />
    <p v-if="config?.accessInstructions" class="instructions">{{ config.accessInstructions }}</p>
    <UiNotice v-if="message" variant="error">{{ message }}</UiNotice>
  </LearningEnvironmentPanel>
</template>
<style scoped>
.target-state {
  font-size: 12px;
  color: var(--muted);
}
time {
  font-variant-numeric: tabular-nums;
}
.instructions {
  margin: 12px 0 0;
  white-space: pre-wrap;
  font-size: 13px;
  color: var(--muted);
}
</style>
