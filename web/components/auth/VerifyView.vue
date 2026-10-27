<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from '~/components/ui/Button.vue'
import { useAuth } from '~/composables/useAuth'

const emit = defineEmits<{
  (e: 'switch-mode', mode: 'login' | 'register'): void
}>()

const route = useRoute()
const router = useRouter()
const { verifyEmail } = useAuth()

type VerifyState = 'pending' | 'success' | 'error' | 'missing'

const status = ref<VerifyState>('pending')
const errorMessage = ref('')
const emailParam = computed(() => {
  return typeof route.query.email === 'string' ? route.query.email : ''
})

onMounted(async () => {
  const token =
    typeof route.query.token === 'string' ? route.query.token.trim() : ''
  if (!token) {
    status.value = 'missing'
    errorMessage.value = '请打开验证邮件中的链接，或重新申请验证邮件。'
    return
  }

  status.value = 'pending'
  try {
    await verifyEmail(token)
    status.value = 'success'
  } catch (err: any) {
    status.value = 'error'
    errorMessage.value =
      err?.data?.statusMessage ||
      err?.data?.message ||
      err?.message ||
      '激活凭证无效或已过期，请重新申请注册或登录。'
  }
})

function goToLogin() {
  emit('switch-mode', 'login')
}
</script>

<template>
  <div class="auth-form-wrapper">
    <!-- 校验中 -->
    <div v-if="status === 'pending'" class="verify-state">
      <UiBusyIndicator inline label="邮箱验证" class="verification-busy"/>
      <div class="eyebrow">ACCOUNT ACTIVATION</div>
      <h1 id="form-title">邮箱验证</h1>

    </div>

    <!-- 激活成功 -->
    <div v-else-if="status === 'success'" class="verify-state">
      <div class="success-icon" aria-hidden="true">✓</div>
      <div class="eyebrow">ACTIVATION SUCCESS</div>
      <h1 id="form-title">邮箱验证成功！</h1>
      <p class="intro">
        您的 AttackBox 账号已完成激活。
        <br />
        现在可以登录账号，开始探索学习与实操。
      </p>

      <Button
        variant="primary"
        class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
        @click="goToLogin"
      >
        <span>立即前往登录</span>
      </Button>
    </div>

    <!-- 激活失败或凭证无效 -->
    <div v-else class="verify-state">
      <div class="error-icon" aria-hidden="true">!</div>
      <div class="eyebrow">ACTIVATION FAILED</div>
      <h1 id="form-title">
        {{ status === 'missing' ? '查收验证邮件' : '激活未完成' }}
      </h1>
      <p class="intro error-text">{{ errorMessage }}</p>

      <AuthResendVerification :email="emailParam" />
      <div class="actions-stack">
        <Button
          variant="primary"
          class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4"
          @click="goToLogin"
        >
          <span>尝试直接登录</span>
        </Button>

        <button
          type="button"
          class="back-button"
          @click="$emit('switch-mode', 'register')"
        >
          <span>返回重新注册</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.verify-state {
  animation: fadeIn 0.3s ease;
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
  font-size: 29px;
  letter-spacing: -0.8px;
  font-weight: 600;
  line-height: 1.4;
  margin: 0;
  color: var(--text-main);
}

.intro {
  font-size: 13px;
  color: var(--text-muted);
  margin-top: 10px;
  margin-bottom: 28px;
  line-height: 1.6;
}

.intro.error-text {
  color: var(--color-error);
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
  margin-bottom: 22px;
}

.error-icon {
  width: 42px;
  height: 42px;
  border: 1px solid rgba(229, 115, 115, 0.3);
  border-radius: 12px;
  display: grid;
  place-items: center;
  color: var(--color-error);
  background: rgba(229, 115, 115, 0.1);
  font-size: 20px;
  font-weight: bold;
  margin-bottom: 22px;
}

.verification-busy{font-size:32px;margin-bottom:22px}

.actions-stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
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
  margin-top: 12px;
  cursor: pointer;
  transition: color 0.2s;
}

.back-button:hover {
  color: var(--text-main);
}
</style>
