<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { registrationEmailAllowed } from '~/shared/registration'
import { apiErrorMessage } from '~/utils/api'

const props = withDefaults(defineProps<{ modelValueEmail?: string }>(), { modelValueEmail: '' })
const emit = defineEmits<{
  (e: 'update:modelValueEmail', value: string): void
  (e: 'switch-mode', mode: 'login'): void
}>()
const { register } = useAuth()
const { data: registrationPolicy } = useLazyFetch<{ domains: string[] }>(
  '/api/auth/registration-policy',
  { key: 'auth:registration-policy', server: false }
)
const email = ref(props.modelValueEmail),
  busy = ref(false),
  mounted = ref(false),
  requested = ref(false)
const { remaining, start: startCooldown } = useEmailCooldown(email)
const turnstileToken = ref(''),
  challengeReady = ref(false),
  challengeReset = ref(0)
const emailError = ref(''),
  message = ref(''),
  error = ref('')
watch(remaining, (value, previous) => {
  if (value > 0) requested.value = true
  if (value === 0 && previous > 0) challengeReset.value++
})
onMounted(() => {
  mounted.value = true
})
watch(
  () => props.modelValueEmail,
  value => {
    email.value = value
  }
)
function updateEmail(value: string) {
  email.value = value
  emit('update:modelValueEmail', value)
  emailError.value = ''
  message.value = ''
  error.value = ''
  requested.value = false
}
async function submit() {
  if (!mounted.value || busy.value || !challengeReady.value || remaining.value > 0) return
  const address = email.value.trim().toLowerCase()
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(address)) emailError.value = '请输入有效的邮箱地址。'
  else if (
    registrationPolicy.value &&
    !registrationEmailAllowed(address, registrationPolicy.value.domains)
  )
    emailError.value = '该邮箱域名暂不支持注册。'
  else emailError.value = ''
  if (emailError.value) return
  busy.value = true
  error.value = ''
  message.value = ''
  email.value = address
  emit('update:modelValueEmail', address)
  try {
    const result = await register(address, turnstileToken.value)
    requested.value = true
    message.value = result.message
    startCooldown(address)
  } catch (cause) {
    error.value = apiErrorMessage(cause, '验证邮件发送失败，请稍后重试')
    challengeReset.value++
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <div class="auth-form">
    <div class="eyebrow">ATTACKBOX ACCOUNT</div>
    <h1 class="auth-title" id="form-title">
      {{ requested || remaining > 0 ? '查收验证邮件。' : '从这里，开始。' }}
    </h1>
    <p class="intro">
      {{
        requested || remaining > 0
          ? '打开邮件中的链接，设置密码并完成注册。'
          : '先验证邮箱，再设置密码，开始你的探索。'
      }}
    </p>
    <form
      id="auth-form"
      method="post"
      action="/api/auth/register"
      novalidate
      @submit.prevent="submit"
    >
      <fieldset :disabled="!mounted || busy" class="auth-fields">
        <div class="field">
          <div class="field-head"><label for="email">邮箱</label></div>
          <UiInput
            id="email"
            name="email"
            type="email"
            inputmode="email"
            autocapitalize="none"
            autocorrect="off"
            spellcheck="false"
            autocomplete="email"
            placeholder="you@example.com"
            :model-value="email"
            :error="emailError"
            required
            maxlength="254"
            @update:model-value="updateEmail"
          />
        </div>
        <AuthChallenge
          v-model="turnstileToken"
          v-model:ready="challengeReady"
          action="register"
          :reset-key="challengeReset"
          :disabled="busy || remaining > 0"
        />
        <UiButton
          variant="primary"
          type="submit"
          :loading="busy"
          :disabled="!challengeReady || remaining > 0"
          class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
        >
          {{
            remaining > 0
              ? `${remaining} 秒后重新发送`
              : requested
                ? '重新发送验证邮件'
                : '发送验证邮件'
          }}
        </UiButton>
        <p v-if="error || message" id="status" class="status" role="status" aria-live="polite">
          {{ error || message }}
        </p>
      </fieldset>
    </form>
    <button class="back-button" type="button" @click="$emit('switch-mode', 'login')">
      返回登录
    </button>
  </div>
</template>
