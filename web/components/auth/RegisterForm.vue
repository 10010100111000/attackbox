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
    emailError.value = '该邮箱域名暂不支持注册，请查看支持的邮箱。'
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
  if (busy.value) return
  if (!validate()) return

  const cleanEmail = email.value.trim()
  email.value = cleanEmail
  emit('update:modelValueEmail', cleanEmail)

  busy.value = true
  statusMessage.value = ''
  try {
    const res = await register(cleanEmail, password.value)
    password.value = ''
    hasRegistered.value = true
    statusMessage.value = res.message
  } catch (err: any) {
    statusMessage.value =
      err?.data?.message || err?.data?.statusMessage || err?.message || '注册失败，请稍后重试'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="auth-form-wrapper">
    <!-- 注册完成后引导用户查收邮件 -->
    <div v-if="hasRegistered" class="success-content">
      <div class="success-icon" aria-hidden="true">✓</div>
      <div class="eyebrow">REGISTRATION SUCCESS</div>
      <h1 id="form-title" tabindex="-1">账号创建成功</h1>
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
      <h1 id="form-title">从这里，开始。</h1>
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

          <div class="registration-policy"><details v-if="registrationPolicy"><summary>支持的邮箱</summary><p>{{registrationPolicy.domains.join(' · ')}}</p></details></div>
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

          <!-- 注册提交按钮 -->
          <Button
            variant="primary"
            :loading="busy"
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
      <AuthResendVerification :email="email" />

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
.auth-fields {
  border: 0;
  margin: 0;
  padding: 0;
  min-width: 0;
}

.success-icon {
  width: 42px;
  height: 42px;
  border: 1px solid rgba(188, 163, 131, 0.25);
  border-radius: 12px;
  display: grid;
  place-items: center;
  color: var(--brand);
  background: var(--brand-soft);
  font-size: 20px;
  margin-bottom: 23px;
}
.eyebrow {
  font:
    9px 'SFMono-Regular',
    Consolas,
    monospace;
  color: var(--text-muted);
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
  color: var(--text-muted);
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
  color: var(--text-main);
}

.hint {
  font-size: 10px;
  line-height: 1.9;
  color: #736a7c;
  margin-top: 15px;
}
</style>

<style scoped>
.registration-policy { min-height: 26px; margin: -8px 0 12px; }
.registration-policy details { font-size: 11px; color: var(--text-muted); }
.registration-policy summary { cursor: pointer; width: fit-content; }
.registration-policy p { max-height: 140px; overflow: auto; line-height: 1.9; overflow-wrap: anywhere; padding: 10px 12px; margin: 8px 0 0; border: 1px solid var(--border-line); border-radius: 6px; background: var(--bg-card); }
.registration-policy summary:focus-visible { outline: 2px solid var(--accent-coral); outline-offset: 3px; }
</style>
