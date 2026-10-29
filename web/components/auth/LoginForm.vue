<script setup lang="ts">
import { ref, watch, nextTick, onMounted } from 'vue'
import { navigateTo, useRoute } from '#imports'
import { loginDestination } from '~/utils/login-destination'
import Button from '~/components/ui/Button.vue'
import Input from '~/components/ui/Input.vue'
import Checkbox from '~/components/ui/Checkbox.vue'

interface Props {
  modelValueEmail?: string
}

const props = withDefaults(defineProps<Props>(), {
  modelValueEmail: ''
})

const emit = defineEmits<{
  (e: 'update:modelValueEmail', value: string): void
  (e: 'switch-mode', mode: 'register' | 'recover'): void
}>()

const mounted = ref(false)
onMounted(() => { mounted.value = true })

const email = ref(props.modelValueEmail)
const password = ref('')
const rememberMe = ref(false)
const busy = ref(false)
const turnstileToken = ref('')
const challengeReady = ref(false)
const challengeReset = ref(0)
const emailError = ref('')
const passwordError = ref('')
const statusMessage = ref('')

watch(() => props.modelValueEmail, (val) => {
  if (val !== undefined && val !== email.value) {
    email.value = val
  }
})

function onEmailInput(val: string) {
  email.value = val
  emit('update:modelValueEmail', val)
  if (emailError.value) emailError.value = ''
  if (statusMessage.value) statusMessage.value = ''
}

function onPasswordInput(val: string) {
  password.value = val
  if (passwordError.value) passwordError.value = ''
  if (statusMessage.value) statusMessage.value = ''
}

function validate(): boolean {
  let firstErrorId: string | null = null
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
  const trimmedEmail = email.value.trim()

  if (!trimmedEmail || !emailRegex.test(trimmedEmail)) {
    emailError.value = '请输入有效的邮箱地址。'
    firstErrorId = 'email'
  } else {
    emailError.value = ''
  }

  if (!password.value) {
    passwordError.value = '请输入密码。'
    if (!firstErrorId) firstErrorId = 'password'
  } else if (password.value.length < 6) {
    passwordError.value = '密码至少需要 6 位。'
    if (!firstErrorId) firstErrorId = 'password'
  } else {
    passwordError.value = ''
  }

  if (firstErrorId) {
    nextTick(() => {
      document.getElementById(firstErrorId)?.focus()
    })
    return false
  }

  return true
}

const { login } = useAuth()
const route = useRoute()

async function handleSubmit() {
  if (!mounted.value || busy.value || !challengeReady.value) return
  if (!validate()) return

  const cleanEmail = email.value.trim()
  email.value = cleanEmail
  emit('update:modelValueEmail', cleanEmail)

  busy.value = true
  statusMessage.value = ''
  try {
    await login(cleanEmail, password.value, rememberMe.value, turnstileToken.value)
    password.value = ''
    await navigateTo(loginDestination(route.query.returnTo), { replace: true })
  } catch (err: any) {
    statusMessage.value = err?.data?.message || err?.data?.statusMessage || err?.message || '登录失败，请检查邮箱与密码'
  } finally {
    busy.value = false
    challengeReset.value++
  }
}


</script>

<template>
  <div class="auth-form-wrapper">
    <div>
      <div class="eyebrow">YOUR NEXT DISCOVERY</div>
      <h1 id="form-title">欢迎回来。</h1>
      <p class="intro">登录，继续你的学习与探索。</p>

      <form id="auth-form" method="post" action="/api/auth/login" novalidate @submit.prevent="handleSubmit">
        <fieldset :disabled="!mounted || busy" class="login-fields">
        <!-- 邮箱字段 -->
        <div class="field">
          <div class="field-head">
            <label for="email">邮箱</label>
          </div>
          <Input
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
            @update:model-value="onEmailInput"
          />
        </div>

        <!-- 密码字段 -->
        <div class="field">
          <div class="field-head">
            <label for="password">密码</label>
            <button
              class="field-link"
              type="button"
              @click="$emit('switch-mode', 'recover')"
            >
              忘记密码？
            </button>
          </div>
          <Input
            id="password"
            name="password"
            type="password"
            autocomplete="current-password"
            placeholder="输入你的密码"
            :model-value="password"
            :error="passwordError"
            required
            @update:model-value="onPasswordInput"
          />
        </div>

        <!-- 保持登录状态 -->
        <label class="remember">
          <Checkbox
            id="remember"
            name="remember"
            :model-value="rememberMe"
            @update:model-value="(val) => rememberMe = val"
          />
          <span>保持登录状态</span>
        </label>

        <AuthChallenge v-model="turnstileToken" v-model:ready="challengeReady" action="login" :reset-key="challengeReset" :disabled="busy" />
        <!-- 登录提交按钮 -->
        <Button
          variant="primary"
          :loading="busy"
          :disabled="!challengeReady"
          type="submit"
          class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
        >
          <span>登录</span>

        </Button>

        <!-- 状态反馈提示 -->
        <p
          v-if="statusMessage"
          id="status"
          class="status"
          role="status"
          aria-live="polite"
        >
          {{ statusMessage }}
        </p>
        </fieldset>
      </form>

      <!-- 底部切换注册链接 -->
      <div class="signup">
        还没有账号？
        <button
          class="text-button"
          type="button"
          @click="$emit('switch-mode', 'register')"
        >
          创建账号
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.eyebrow {
  font: 9px "SFMono-Regular", Consolas, monospace;
  color: var(--brand-warm);
  letter-spacing: 2px;
  margin-bottom: 16px;
}

h1 {
  font-size: 32px;
  letter-spacing: -1px;
  font-weight: 550;
  line-height: 1.4;
  margin: 0;
}

.intro {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 10px;
  margin-bottom: 31px;
}

.field {
  margin-bottom: 20px;
}

.field-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 9px;
}

.field label {
  font-size: 11px;
  color: var(--text-main);
}

.field-link,
.text-button {
  background: none;
  border: 0;
  padding: 0;
  color: var(--text-muted);
  font-size: 10px;
  cursor: pointer;
  transition: color 0.2s;
}

.field-link:hover,
.text-button:hover {
  color: var(--text-main);
}

.remember {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
  font-size: 10px;
  width: fit-content;
  margin: 2px 0 24px;
  cursor: pointer;
  user-select: none;
}

.signup {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  margin-top: 28px;
  font-size: 11px;
  color: var(--text-muted);
}

.signup .text-button {
  font-size: 11px;
  color: var(--text-main);
}

.status {
  font-size: 11px;
  color: var(--brand-warm);
  line-height: 1.8;
  padding: 12px 14px;
  border: 1px solid rgba(201, 160, 124, 0.13);
  background: rgba(177, 132, 102, 0.06);
  border-radius: 7px;
  margin-top: 17px;
}

.login-fields {
  border: 0;
  margin: 0;
  padding: 0;
  min-width: 0;
}
</style>
