<script setup lang="ts">
const props = defineProps<{ email?: string }>()
const address = ref(props.email || ''),
  busy = ref(false),
  message = ref('')
const id = useId()
watch(
  () => props.email,
  email => {
    if (email) address.value = email
  }
)
async function resend() {
  if (busy.value) return
  busy.value = true
  message.value = ''
  try {
    const result = await $fetch<{ message: string }>('/api/auth/resend-verification', {
      method: 'POST',
      body: { email: address.value },
      retry: false
    })
    message.value = result.message
  } catch (e: any) {
    message.value = e?.data?.statusMessage || '邮件发送暂不可用，请稍后重试'
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <form
    method="post"
    action="/api/auth/resend-verification"
    class="resend-verification"
    @submit.prevent="resend"
  >
    <label :for="id">重新发送验证邮件</label>
    <div class="input-row">
      <UiInput
        :id="id"
        v-model="address"
        name="email"
        type="email"
        required
        maxlength="254"
        autocomplete="email"
        placeholder="注册邮箱"
        :disabled="busy"
        class="flex-1"
      />
      <UiButton type="submit" variant="secondary" :loading="busy" :disabled="busy" class="shrink-0">
        发送验证邮件
      </UiButton>
    </div>
    <UiNotice v-if="message" variant="info" class="mt-3">
      {{ message }}
    </UiNotice>
  </form>
</template>
<style scoped>
.resend-verification {
  margin: 24px 0;
  text-align: left;
}
.resend-verification label {
  display: block;
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 10px;
}
.input-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
</style>
