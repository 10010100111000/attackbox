<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAuth } from '~/composables/useAuth'
import { apiFetch, apiErrorMessage } from '~/utils/api'

const emit = defineEmits<{
  (e: 'switch-mode', mode: 'login' | 'register'): void
  (e: 'update:modelValueEmail', email: string): void
}>()
const route = useRoute(),
  { verifyEmail } = useAuth()
const token = typeof route.query.token === 'string' ? route.query.token : ''
const status = ref<'loading' | 'form' | 'success' | 'error'>('loading')
const email = ref(''),
  password = ref(''),
  error = ref(''),
  busy = ref(false)
onMounted(async () => {
  if (!token) {
    status.value = 'error'
    error.value = '请打开验证邮件中的链接，或重新申请验证邮件。'
    return
  }
  try {
    const result = await apiFetch<{ email: string }>('/api/auth/registration-token', {
      method: 'POST',
      body: { token }
    })
    email.value = result.email
    emit('update:modelValueEmail', result.email)
    status.value = 'form'
  } catch (cause) {
    error.value = apiErrorMessage(cause, '验证链接无效或已过期，请重新申请注册。')
    status.value = 'error'
  }
})
async function complete() {
  if (busy.value || status.value !== 'form') return
  if (password.value.length < 6 || password.value.length > 1024) {
    error.value = '密码长度需为 6–1024 位。'
    return
  }
  busy.value = true
  error.value = ''
  try {
    await verifyEmail(token, password.value)
    password.value = ''
    status.value = 'success'
  } catch (cause) {
    error.value = apiErrorMessage(cause, '暂时无法完成注册，请重试。')
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <div class="auth-form">
    <template v-if="status === 'loading'">
      <UiBusyIndicator inline label="邮箱验证" />
      <h1 class="auth-title" id="form-title">邮箱验证</h1>
    </template>
    <template v-else-if="status === 'form'">
      <div class="eyebrow">ONE LAST STEP</div>
      <h1 class="auth-title" id="form-title">设置你的密码。</h1>
      <p class="intro">{{ email }}</p>
      <form
        id="auth-form"
        method="post"
        action="/api/auth/verify-email"
        novalidate
        @submit.prevent="complete"
      >
        <fieldset :disabled="busy" class="auth-fields">
          <div class="field">
            <div class="field-head"><label for="password">密码</label></div>
            <UiInput
              id="password"
              v-model="password"
              name="password"
              type="password"
              autocomplete="new-password"
              placeholder="设置密码，至少 6 位"
              minlength="6"
              maxlength="1024"
              required
            />
          </div>
          <UiButton
            type="submit"
            variant="primary"
            :loading="busy"
            class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
            >完成注册</UiButton
          >
          <p v-if="error" class="status" role="status" aria-live="polite">{{ error }}</p>
        </fieldset>
      </form>
    </template>
    <template v-else-if="status === 'success'">
      <div class="success-icon" aria-hidden="true">✓</div>
      <h1 class="auth-title" id="form-title">账号创建成功。</h1>
      <p class="intro">邮箱已验证，使用刚设置的密码开始学习。</p>
      <UiButton
        variant="primary"
        class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
        @click="$emit('switch-mode', 'login')"
        >前往登录</UiButton
      >
    </template>
    <template v-else>
      <h1 class="auth-title" id="form-title">重新验证邮箱。</h1>
      <p class="intro">{{ error }}</p>
      <UiButton
        variant="primary"
        class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
        @click="$emit('switch-mode', 'register')"
        >重新申请验证邮件</UiButton
      >
      <button class="back-button" type="button" @click="$emit('switch-mode', 'login')">
        返回登录
      </button>
    </template>
  </div>
</template>
