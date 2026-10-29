<script setup lang="ts">
import { ref, watch, nextTick, onMounted } from 'vue'
import Button from '~/components/ui/Button.vue'
import Input from '~/components/ui/Input.vue'
import { registrationEmailAllowed } from '~/shared/registration'

interface Props {
  modelValueEmail?: string
}

const props = withDefaults(defineProps<Props>(), {
  modelValueEmail: '',
})

const emit = defineEmits<{
  (e: 'update:modelValueEmail', value: string): void
  (e: 'switch-mode', mode: 'login'): void
}>()

const { data: registrationPolicy } = useLazyFetch<{domains:string[]}>('/api/auth/registration-policy', { key:'auth:registration-policy', server:false })
const email = ref(props.modelValueEmail)
const password = ref('')
const busy = ref(false)
const turnstileToken = ref('')
const challengeReady = ref(false)
const challengeReset = ref(0)
const mounted = ref(false)
onMounted(() => {
  mounted.value = true
})
const hasRegistered = ref(false)
const emailError = ref('')
const passwordError = ref('')
const statusMessage = ref('')

watch(
  () => props.modelValueEmail,
  (val) => {
    if (val !== undefined && val !== email.value) {
      email.value = val
    }
  },
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
  } else if (registrationPolicy.value && !registrationEmailAllowed(trimmedEmail, registrationPolicy.value.domains)) {
    emailError.value = '该邮箱域名暂不支持注册。'
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

const { register } = useAuth()

async function handleSubmit() {
  if (busy.value || !challengeReady.value) return
  if (!validate()) return

  const cleanEmail = email.value.trim()
  email.value = cleanEmail
  emit('update:modelValueEmail', cleanEmail)

  busy.value = true
  statusMessage.value = ''
  try {
    const res = await register(cleanEmail, password.value, undefined, turnstileToken.value)
    password.value = ''
    hasRegistered.value = true
    statusMessage.value = res.message
  } catch (err: any) {
    statusMessage.value =
      err?.data?.message || err?.data?.statusMessage || err?.message || '注册失败，请稍后重试'
  } finally {
    busy.value = false
    challengeReset.value++
  }
}
</script>

<template>
  <div class="auth-form">
    <!-- 注册完成后引导用户查收邮件 -->
    <div v-if="hasRegistered" class="success-content">
      <div class="success-icon" aria-hidden="true">✓</div>
      <div class="eyebrow">REGISTRATION SUCCESS</div>
      <h1 class="auth-title" id="form-title" tabindex="-1">账号创建成功</h1>
      <p class="intro">
        验证邮件已发送，请打开邮件中的链接完成激活。
        <br />
        没有收到时，请检查垃圾邮件或重新发送。
      </p>
      <AuthResendVerification :email="email" />
      <button
        type="button"
        class="back-button"
        @click="$emit('switch-mode', 'login')"
      >
        <span>返回登录</span>
      </button>
    </div>

    <!-- 正常注册表单 -->
    <div v-else>
      <div class="eyebrow">ATTACKBOX ACCOUNT</div>
      <h1 class="auth-title" id="form-title">从这里，开始。</h1>
      <p class="intro">创建账号，开始你的下一次探索。</p>

      <form
        id="auth-form"
        method="post"
        action="/api/auth/register"
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
            </div>
            <Input
              id="password"
              name="password"
              type="password"
              autocomplete="new-password"
              placeholder="设置密码，至少 6 位"
              :model-value="password"
              :error="passwordError"
              required
              @update:model-value="onPasswordInput"
            />
          </div>

        <AuthChallenge v-model="turnstileToken" v-model:ready="challengeReady" action="register" :reset-key="challengeReset" :disabled="busy" />
          <!-- 注册提交按钮 -->
          <Button
            variant="primary"
            :loading="busy"
          :disabled="!challengeReady"
            type="submit"
            class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
          >
            <span>继续创建账号</span>
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

      <!-- 返回登录 -->
      <button
        class="back-button"
        type="button"
        @click="$emit('switch-mode', 'login')"
      >
        <span>返回登录</span>
      </button>

      <!-- 提示语 -->
      <p class="hint">下一步验证邮箱，完成账号创建。</p>
    </div>
  </div>
</template>

<style scoped>
.hint {
  font-size: 10px;
  line-height: 1.9;
  color: #736a7c;
  margin-top: 15px;
}
</style>
