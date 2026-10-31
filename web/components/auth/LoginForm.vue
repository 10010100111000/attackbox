<script setup lang="ts">
import { ref, watch, nextTick, onMounted } from 'vue'
import { navigateTo, useRoute } from '#imports'
import { loginDestination } from '~/utils/login-destination'
import Button from '~/components/ui/Button.vue'
import Input from '~/components/ui/Input.vue'
import Checkbox from '~/components/ui/Checkbox.vue'
import { apiErrorMessage } from '~/utils/api'

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
onMounted(() => {
  mounted.value = true
})

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

watch(
  () => props.modelValueEmail,
  val => {
    if (val !== undefined && val !== email.value) {
      email.value = val
    }
  }
)

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
  } catch (err) {
    statusMessage.value = apiErrorMessage(err, '登录失败，请检查邮箱与密码')
    challengeReset.value++
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="auth-form">
    <div>
      <div class="eyebrow">YOUR NEXT DISCOVERY</div>
      <h1 class="auth-title" id="form-title">欢迎回来。</h1>
      <p class="intro">登录，继续你的学习与探索。</p>

      <form
        id="auth-form"
        method="post"
        action="/api/auth/login"
        novalidate
        @submit.prevent="handleSubmit"
      >
        <fieldset :disabled="!mounted || busy" class="auth-fields">
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
              <button class="field-link" type="button" @click="$emit('switch-mode', 'recover')">
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
              @update:model-value="val => (rememberMe = val)"
            />
            <span>保持登录状态</span>
          </label>

          <AuthChallenge
            v-model="turnstileToken"
            v-model:ready="challengeReady"
            action="login"
            :reset-key="challengeReset"
            :disabled="busy"
          />
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
          <p v-if="statusMessage" id="status" class="status" role="status" aria-live="polite">
            {{ statusMessage }}
          </p>
        </fieldset>
      </form>

      <!-- 底部切换注册链接 -->
      <div class="signup">
        还没有账号？
        <button class="text-button" type="button" @click="$emit('switch-mode', 'register')">
          创建账号
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
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
</style>
