<script setup lang="ts">
import { ref, watch, nextTick, onMounted } from 'vue'
import Button from '~/components/ui/Button.vue'
import Input from '~/components/ui/Input.vue'

interface Props {
  modelValueEmail?: string
}

const props = withDefaults(defineProps<Props>(), {
  modelValueEmail: ''
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
    statusMessage.value = res?.message || '重置密码链接已发送至你的邮箱，请查收邮件。'
  } catch (err) {
    statusMessage.value = apiErrorMessage(err, '发送失败，请稍后重试')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="auth-form">
    <div class="eyebrow">ATTACKBOX ACCOUNT</div>
    <h1 class="auth-title" id="form-title">找回密码。</h1>
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
            {{ hasSent ? '再次发送重置链接' : '发送重置链接' }}
          </span>
        </Button>

        <!-- 状态反馈提示 -->
        <p v-if="statusMessage" id="status" class="status" role="status" aria-live="polite">
          {{ statusMessage }}
        </p>
      </fieldset>
    </form>

    <!-- 返回登录 -->
    <button class="back-button" type="button" @click="$emit('switch-mode', 'login')">
      <span>返回登录</span>
    </button>
  </div>
</template>
