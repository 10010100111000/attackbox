<script setup lang="ts">
import { Check } from 'lucide-vue-next'

const emit = defineEmits<{ close: [] }>()
const { currentUser } = useAuth()
const current = ref('')
const password = ref('')
const confirmation = ref('')
const busy = ref(false)
const error = ref('')
const message = ref('')
let alive = true

function clear() {
  current.value = ''
  password.value = ''
  confirmation.value = ''
}

watch(
  () => currentUser.value?.id,
  () => {
    clear()
    message.value = ''
    error.value = ''
  }
)

onBeforeUnmount(() => {
  alive = false
  clear()
})

async function save() {
  if (busy.value || !currentUser.value) return
  error.value = ''
  message.value = ''
  if (password.value !== confirmation.value) {
    error.value = '两次新密码不一致'
    return
  }
  if (password.value.length < 6) {
    error.value = '新密码至少需要 6 位'
    return
  }
  const owner = currentUser.value.id
  busy.value = true
  try {
    await $fetch('/api/me/password', {
      method: 'PUT',
      body: { currentPassword: current.value, newPassword: password.value },
      retry: false
    })
    if (!alive || currentUser.value?.id !== owner) return
    clear()
    message.value = '密码已更新，其他设备需重新登录。'
  } catch (e: any) {
    if (alive && currentUser.value?.id === owner) {
      error.value = e?.data?.message || '修改失败，请重试'
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="password-settings" @submit.prevent="save">
    <fieldset :disabled="busy">
      <div class="personal-field">
        <label for="account-current-password">当前密码</label>
        <input
          id="account-current-password"
          v-model="current"
          type="password"
          autocomplete="current-password"
          maxlength="1024"
          required
        />
      </div>
      <div class="personal-field">
        <label for="account-new-password">新密码</label>
        <input
          id="account-new-password"
          v-model="password"
          type="password"
          autocomplete="new-password"
          minlength="6"
          maxlength="1024"
          placeholder="至少 6 位"
          required
        />
      </div>
      <div class="personal-field">
        <label for="account-confirm-password">确认新密码</label>
        <input
          id="account-confirm-password"
          v-model="confirmation"
          type="password"
          autocomplete="new-password"
          minlength="6"
          maxlength="1024"
          required
        />
      </div>
    </fieldset>
    <div class="personal-save-row">
      <p v-if="error" role="alert" class="personal-field-error">{{ error }}</p>
      <p v-else-if="message" role="status" class="personal-save-success">
        <Check :size="15" />
        {{ message }}
      </p>
      <span v-else />
      <div class="personal-save-actions">
        <button
          type="button"
          class="personal-button secondary"
          :disabled="busy"
          @click="emit('close')"
        >
          取消修改密码
        </button>
        <button class="personal-button" :disabled="busy || !current || !password || !confirmation">
          <UiBusyLabel :busy="busy">更新密码</UiBusyLabel>
        </button>
      </div>
    </div>
  </form>
</template>

<style scoped>
.password-settings fieldset {
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
}
</style>
