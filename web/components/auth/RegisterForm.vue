<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight, ArrowLeft } from 'lucide-vue-next'
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

const router = useRouter()
const email = ref(props.modelValueEmail)
const password = ref('')
const busy = ref(false)
const isSuccessPreview = ref(false)
const generatedToken = ref('')
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
    if (res?.verifyToken) {
      generatedToken.value = res.verifyToken
      isSuccessPreview.value = true
    } else {
      statusMessage.value = res?.message || '邮箱验证邮件已发送，请查收邮件以完成账号激活。'
    }
  } catch (err: any) {
    statusMessage.value = err?.data?.statusMessage || err?.message || '注册失败，请稍后重试'
  } finally {
    busy.value = false
  }
}

function handleActivateNow() {
  router.push({
    path: '/auth',
    query: {
      mode: 'verify',
      token: generatedToken.value,
      email: email.value
    }
  })
}
</script>

<template>
  <div class="auth-form-wrapper">
    <!-- 注册成功后的凭据与激活视图 -->
    <div v-if="isSuccessPreview" class="success-content">
      <div class="success-icon" aria-hidden="true">✓</div>
      <div class="eyebrow">REGISTRATION SUCCESS</div>
      <h1 id="form-title" tabindex="-1">账号创建成功</h1>
      <p class="intro">
        账号注册已提交，激活凭据已生成。<br />
        点击下方按钮立即完成邮箱验证并激活账号。
      </p>
      <Button
        variant="primary"
        class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
        @click="handleActivateNow"
      >
        <span>一键激活并登录</span>
        <ArrowRight class="w-4 h-4" />
      </Button>
      <button
        type="button"
        class="back-button"
        @click="$emit('switch-mode', 'login')"
      >
        <ArrowLeft class="w-3.5 h-3.5 inline mr-1" />
        <span>返回登录</span>
      </button>
    </div>

    <!-- 正常注册表单 -->
    <div v-else>
      <div class="eyebrow">ATTACKBOX ACCOUNT</div>
      <h1 id="form-title">从这里，开始。</h1>
      <p class="intro">创建账号，开始你的下一次探索。</p>

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
        <span>{{ busy ? '正在继续…' : '继续创建账号' }}</span>
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

    <!-- 返回登录 -->
    <button
      class="back-button"
      type="button"
      @click="$emit('switch-mode', 'login')"
    >
      <ArrowLeft class="w-3.5 h-3.5 inline mr-1" />
      <span>返回登录</span>
    </button>

    <!-- 提示语 -->
    <p class="hint">下一步验证邮箱，完成账号创建。</p>
    </div>
  </div>
</template>

<style scoped>
.success-icon {
  width: 42px;
  height: 42px;
  border: 1px solid rgba(188, 163, 131, 0.25);
  border-radius: 12px;
  display: grid;
  place-items: center;
  color: #8ed6bd;
  background: rgba(142, 214, 189, 0.1);
  font-size: 20px;
  margin-bottom: 23px;
}
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

.arrow {
  font-size: 15px;
  display: inline-block;
  transition: transform 0.2s;
}

.submit:hover .arrow {
  transform: translate(2px, -2px);
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

.hint {
  font-size: 10px;
  line-height: 1.9;
  color: #736a7c;
  margin-top: 15px;
}
</style>
