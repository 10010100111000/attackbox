<script setup lang="ts">
import { loadTurnstile, turnstileApi } from '~/utils/turnstile-client'

type AuthChallengeAction = 'login' | 'register'
const props = defineProps<{
  action: AuthChallengeAction
  resetKey: number
  modelValue?: string
  disabled?: boolean
}>()
const emit = defineEmits<{
  'update:modelValue': [value: string]
  'update:ready': [value: boolean]
}>()
const { currentTheme } = useTheme()
const container = ref<HTMLElement | null>(null)
const message = ref(''), loading = ref(false)
const configuration = ref<{ enabled: boolean; siteKey: string }>()
let widget: string | undefined, alive = true, generation = 0

function invalidate(text = '') {
  emit('update:modelValue', '')
  emit('update:ready', false)
  message.value = text
}

async function setup() {
  const ownGeneration = ++generation
  invalidate()
  loading.value = true
  if (widget) turnstileApi()?.remove(widget)
  widget = undefined
  try {
    const config = await $fetch<{ enabled: boolean; siteKey: string }>('/api/auth/challenge', { retry: false, timeout: 10000 })
    if (!alive || generation !== ownGeneration) return
    configuration.value = config
    if (config?.enabled === false) { emit('update:ready', true); return }
    if (config?.enabled !== true || !config.siteKey) throw Error('configuration unavailable')
    await loadTurnstile()
    await nextTick()
    if (!alive || generation !== ownGeneration || !container.value) return
    widget = turnstileApi()!.render(container.value, {
      sitekey: config.siteKey,
      action: props.action,
      theme: currentTheme.value === 'midnight' ? 'dark' : 'light',
      size: 'flexible',
      callback: (token: string) => {
        if (!alive || generation !== ownGeneration) return
        message.value = ''
        emit('update:modelValue', token)
        emit('update:ready', true)
      },
      'expired-callback': () => {
        if (alive && generation === ownGeneration) invalidate('验证已过期，请重新验证。')
      },
      'error-callback': () => {
        if (alive && generation === ownGeneration) invalidate('验证暂不可用，请重试。')
      },
    })
  } catch {
    if (alive && generation === ownGeneration) invalidate('人机验证加载失败，请重试。')
  } finally {
    if (alive && generation === ownGeneration) loading.value = false
  }
}

function reset() {
  if (!configuration.value?.enabled) return
  invalidate()
  if (widget) turnstileApi()?.reset(widget)
}
watch(() => props.resetKey, reset)
watch(currentTheme, () => { if (configuration.value?.enabled) void setup() })
onMounted(setup)
onBeforeUnmount(() => {
  alive = false
  generation++
  if (widget) turnstileApi()?.remove(widget)
})
</script>

<template>
  <div class="auth-challenge" :class="{ visible: configuration?.enabled || loading || message }" :inert="disabled || undefined">
    <div ref="container" />
    <UiBusyIndicator v-if="loading" inline label="人机验证加载" />
    <div v-if="message" class="challenge-error">
      <span role="alert">{{ message }}</span>
      <button type="button" :disabled="loading" @click="setup">重试验证</button>
    </div>
  </div>
</template>

<style scoped>
.auth-challenge { display: none; }
.auth-challenge.visible { display: grid; gap: 8px; margin: 0 0 18px; min-width: 0; }
.auth-challenge > div { max-width: 100%; }
.challenge-error { display: flex; align-items: center; gap: 10px; font-size: 12px; line-height: 1.8; color: var(--color-error); }
.challenge-error button { flex-shrink: 0; background: none; border: 0; padding: 2px; color: var(--text-main); text-decoration: underline; cursor: pointer; }
.challenge-error button:focus-visible { outline: 2px solid var(--accent-coral); outline-offset: 3px; }
</style>
