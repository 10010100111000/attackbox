<script setup lang="ts">
import { ref, watch, nextTick, onMounted } from 'vue'
import Button from '~/components/ui/Button.vue'
import Input from '~/components/ui/Input.vue'

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

const email = ref(props.modelValueEmail)
const busy = ref(false)
const mounted = ref(false)
onMounted(() => {
  mounted.value = true
})
const hasSent = ref(false)
const emailError = ref('')
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

function validate(): boolean {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
  const trimmedEmail = email.value.trim()

  if (!trimmedEmail || !emailRegex.test(trimmedEmail)) {
    emailError.value = '请输入有效的邮箱地址。'
    nextTick(() => {
      document.getElementById('email')?.focus()
    })
    return false
  }

  emailError.value = ''
  return true
}

const { forgotPassword } = useAuth()

async function handleSubmit() {
  if (busy.value) return
  if (!validate()) return

  const cleanEmail = email.value.trim()
  email.value = cleanEmail
  emit('update:modelValueEmail', cleanEmail)

  busy.value = true
  statusMessage.value = ''
  try {
    const res = await forgotPassword(cleanEmail)
    hasSent.value = true
    statusMessage.value =
      res?.message || '重置密码链接已发送至你的邮箱，请查收邮件。'
  } catch (err: any) {
    statusMessage.value =
      err?.data?.statusMessage || err?.message || '发送失败，请稍后重试'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="auth-form-wrapper">
    <div class="eyebrow">ATTACKBOX ACCOUNT</div>
    <h1 id="form-title">找回密码。</h1>
    <p class="intro">输入注册邮箱，我们将帮助你重设密码。</p>

    <form
      id="auth-form"
      method="post"
      action="/api/auth/forgot-password"
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

        <!-- 发送重置链接按钮 -->
        <Button
          variant="primary"
          :loading="busy"
          type="submit"
          class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
        >
          <span>
            {{
              busy ? '正在继续…' : hasSent ? '再次发送重置链接' : '发送重置链接'
            }}
          </span>
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
  </div>
</template>

<style scoped>
.auth-fields {
  border: 0;
  margin: 0;
  padding: 0;
  min-width: 0;
}

.eyebrow {
  font:
    9px 'SFMono-Regular',
    Consolas,
    monospace;
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
</style>
