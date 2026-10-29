<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from '#imports'
import { useAuth } from '~/composables/useAuth'
import Button from '~/components/ui/Button.vue'
import Input from '~/components/ui/Input.vue'

const emit = defineEmits<{
  'switch-mode': [mode: 'login' | 'recover']
}>()

const route = useRoute()
const { resetPassword } = useAuth()

const token = computed(() => (typeof route.query.token === 'string' ? route.query.token : ''))
const password = ref('')
const confirmation = ref('')
const busy = ref(false)
const done = ref(false)
const error = ref('')

async function submit() {
  if (busy.value) return
  error.value = ''
  if (!token.value) {
    error.value = '链接缺少重置凭证，请重新申请。'
    return
  }
  if (password.value.length < 6) {
    error.value = '新密码至少需要 6 位。'
    return
  }
  if (password.value !== confirmation.value) {
    error.value = '两次输入的密码不一致。'
    return
  }
  busy.value = true
  try {
    await resetPassword(token.value, password.value)
    done.value = true
    password.value = ''
    confirmation.value = ''
  } catch (e: any) {
    error.value = e?.data?.message || e?.data?.statusMessage || '重设失败，请重新申请重置链接。'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="auth-form-wrapper">
    <div class="eyebrow">ATTACKBOX ACCOUNT</div>
    <h1 id="form-title">重设密码。</h1>
    <p class="intro">为你的账号设置新密码以恢复访问。</p>

    <template v-if="done">
      <div class="status" role="status">密码已更新，请使用新密码登录。</div>
      <Button
        variant="primary"
        class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4 mt-6"
        @click="emit('switch-mode', 'login')"
      >
        <span>返回登录</span>
        <span class="arrow" aria-hidden="true">→</span>
      </Button>
    </template>

    <form v-else id="auth-form" @submit.prevent="submit">
      <fieldset :disabled="busy" class="auth-fields">
        <div class="field">
          <div class="field-head">
            <label for="reset-password">新密码</label>
          </div>
          <Input
            id="reset-password"
            v-model="password"
            type="password"
            autocomplete="new-password"
            placeholder="至少 6 位字符"
            required
            :disabled="busy"
          />
        </div>

        <div class="field">
          <div class="field-head">
            <label for="reset-confirm">确认密码</label>
          </div>
          <Input
            id="reset-confirm"
            v-model="confirmation"
            type="password"
            autocomplete="new-password"
            placeholder="再次输入新密码"
            required
            :disabled="busy"
          />
        </div>

        <div v-if="error" class="status" role="alert">
          {{ error }}
        </div>

        <Button
          type="submit"
          variant="primary"
          class="submit w-full h-[46px] text-xs font-semibold rounded-md gap-4 mt-6"
          :loading="busy"
          :disabled="busy"
        >
          <span>更新密码</span>
          <span class="arrow" aria-hidden="true">→</span>
        </Button>

        <div class="signup">
          <span>链接已失效？</span>
          <button type="button" class="text-button" :disabled="busy" @click="emit('switch-mode', 'recover')">
            重新申请重置链接
          </button>
        </div>
      </fieldset>
    </form>
  </div>
</template>

<style scoped>
.auth-form-wrapper {
  width: 100%;
}

.eyebrow {
  font:
    9px 'SFMono-Regular',
    Consolas,
    monospace;
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

.auth-fields {
  border: 0;
  margin: 0;
  padding: 0;
  min-width: 0;
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
  color: var(--brand-warm);
  line-height: 1.8;
  padding: 12px 14px;
  border: 1px solid rgba(201, 160, 124, 0.13);
  background: rgba(177, 132, 102, 0.06);
  border-radius: 7px;
  margin-top: 17px;
}

.status[role='alert'] {
  color: var(--error-ink);
  border-color: rgba(225, 130, 130, 0.2);
  background: var(--error-bg);
}

.signup {
  margin-top: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  font-size: 11px;
  color: #7f7688;
}

.text-button {
  background: none;
  border: 0;
  padding: 0;
  color: var(--text-main);
  font-size: 11px;
  cursor: pointer;
  transition: color 0.2s;
}

.text-button:hover {
  color: var(--brand-warm);
}

.arrow {
  font-size: 15px;
  display: inline-block;
  transition: transform 0.2s;
}

.submit:hover .arrow {
  transform: translate(2px, -2px);
}
</style>
