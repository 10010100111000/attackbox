<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { ArrowRight, ArrowLeft } from 'lucide-vue-next'
import Button from '~/components/ui/Button.vue'
import Input from '~/components/ui/Input.vue'
import Checkbox from '~/components/ui/Checkbox.vue'
import Separator from '~/components/ui/Separator.vue'

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

const email = ref(props.modelValueEmail)
const password = ref('')
const rememberMe = ref(false)
const busy = ref(false)
const isSuccessPreview = ref(false)
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

async function handleSubmit() {
  if (busy.value) return
  if (!validate()) return

  const cleanEmail = email.value.trim()
  email.value = cleanEmail
  emit('update:modelValueEmail', cleanEmail)

  busy.value = true
  await new Promise((resolve) => setTimeout(resolve, 650))
  busy.value = false
  password.value = ''
  isSuccessPreview.value = true

  nextTick(() => {
    document.getElementById('form-title')?.focus()
  })
}

function handleProvider(_providerName: string) {
  // 单点登录与第三方入口当前点击静默不生效
}

function resetToLogin() {
  isSuccessPreview.value = false
  password.value = ''
  statusMessage.value = ''
  nextTick(() => {
    document.getElementById('email')?.focus()
  })
}
</script>

<template>
  <div class="auth-form-wrapper">
    <!-- 登录成功后的预览视图 -->
    <div v-if="isSuccessPreview" class="success-content">
      <div class="success-icon" aria-hidden="true">✓</div>
      <div class="eyebrow">LOGIN SUCCESS</div>
      <h1 id="form-title" tabindex="-1">准备好，继续探索。</h1>
      <p>登录成功。<br>准备进入学习控制台与实验靶场。</p>
      <Button
        as="NuxtLink"
        to="/"
        variant="primary"
        class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
      >
        <span>进入学习平台</span>
        <ArrowRight class="w-4 h-4" />
      </Button>
      <button
        type="button"
        class="back-button"
        @click="resetToLogin"
      >
        <ArrowLeft class="w-3.5 h-3.5 inline mr-1" />
        <span>返回登录</span>
      </button>
    </div>

    <!-- 正常登录表单 -->
    <div v-else>
      <div class="eyebrow">YOUR NEXT DISCOVERY</div>
      <h1 id="form-title">欢迎回来。</h1>
      <p class="intro">登录，继续你的学习与探索。</p>

      <form id="auth-form" novalidate @submit.prevent="handleSubmit">
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

        <!-- 登录提交按钮 -->
        <Button
          variant="primary"
          :loading="busy"
          type="submit"
          class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
        >
          <span>{{ busy ? '正在继续…' : '登录' }}</span>
          <ArrowRight v-if="!busy" class="w-4 h-4" />
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
      </form>

      <!-- 第三方认证 -->
      <Separator class="my-6">或使用以下方式</Separator>

      <div class="providers">
        <button
          type="button"
          class="provider"
          data-provider="GitHub"
          @click="handleProvider('GitHub')"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M12 .8a11.3 11.3 0 0 0-3.57 22c.56.1.77-.24.77-.54v-2.1c-3.14.68-3.8-1.33-3.8-1.33-.51-1.3-1.26-1.65-1.26-1.65-1.03-.71.08-.69.08-.69 1.14.08 1.74 1.17 1.74 1.17 1 1.73 2.65 1.23 3.3.94.1-.73.4-1.23.72-1.51-2.51-.28-5.15-1.26-5.15-5.59 0-1.24.44-2.25 1.17-3.04-.12-.29-.51-1.44.11-2.99 0 0 .96-.31 3.11 1.16a10.84 10.84 0 0 1 5.66 0c2.16-1.47 3.11-1.16 3.11-1.16.62 1.55.23 2.7.11 2.99.73.79 1.16 1.8 1.16 3.04 0 4.34-2.64 5.3-5.16 5.58.4.35.77 1.04.77 2.1v3.08c0 .3.21.65.78.54A11.3 11.3 0 0 0 12 .8Z"/>
          </svg>
          GitHub
        </button>
        <button
          type="button"
          class="provider"
          data-provider="单点登录"
          @click="handleProvider('单点登录')"
        >
          <span class="sso" aria-hidden="true">⟷</span>
          单点登录
        </button>
      </div>

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
  color: #9c8275;
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
  color: #c2b9c5;
}

.field-link,
.text-button {
  background: none;
  border: 0;
  padding: 0;
  color: #a18d82;
  font-size: 10px;
  cursor: pointer;
  transition: color 0.2s;
}

.field-link:hover,
.text-button:hover {
  color: #e4bd9f;
}

.remember {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #8d8494;
  font-size: 10px;
  width: fit-content;
  margin: 2px 0 24px;
  cursor: pointer;
  user-select: none;
}

.arrow {
  font-size: 15px;
  display: inline-block;
  transition: transform 0.2s;
}

.submit:hover .arrow {
  transform: translate(2px, -2px);
}

.providers {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.provider {
  height: 42px;
  border: 1px solid rgba(255, 255, 255, 0.09);
  border-radius: 7px;
  background: transparent;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  font-size: 10px;
  color: #a79da9;
  cursor: pointer;
  transition: background 0.2s, border-color 0.2s, color 0.2s;
}

.provider:hover {
  background: rgba(255, 255, 255, 0.02);
  border-color: rgba(255, 255, 255, 0.19);
  color: #d4c9d5;
}

.provider svg {
  width: 15px;
  height: 15px;
  fill: currentColor;
}

.provider .sso {
  font: 11px "SFMono-Regular", Consolas, monospace;
  letter-spacing: -2px;
  padding-right: 2px;
}

.signup {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  margin-top: 28px;
  font-size: 11px;
  color: #7f7688;
}

.signup .text-button {
  font-size: 11px;
  color: #d9af96;
}

.status {
  font-size: 11px;
  color: #c5aa97;
  line-height: 1.8;
  padding: 12px 14px;
  border: 1px solid rgba(201, 160, 124, 0.13);
  background: rgba(177, 132, 102, 0.06);
  border-radius: 7px;
  margin-top: 17px;
}

.back-button {
  font-size: 11px;
  color: #ad9487;
  background: none;
  border: 0;
  padding: 0;
  display: flex;
  gap: 9px;
  align-items: center;
  margin-top: 23px;
  cursor: pointer;
  transition: color 0.2s;
}

.back-button:hover {
  color: #e4bd9f;
}

.success-icon {
  width: 42px;
  height: 42px;
  border: 1px solid rgba(188, 163, 131, 0.25);
  border-radius: 12px;
  display: grid;
  place-items: center;
  color: #d7b193;
  font-size: 20px;
  margin-bottom: 23px;
}

.success-content h1 {
  font-size: 29px;
  outline: none;
}

.success-content p {
  margin: 12px 0 27px;
  color: #9d91a5;
  font-size: 12px;
  line-height: 1.9;
}
</style>
