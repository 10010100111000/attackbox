<script setup lang="ts">
const props = defineProps<{ email?: string }>()
const address = ref(props.email || ''),
  busy = ref(false),
  message = ref('')
const id = useId()
watch(
  () => props.email,
  (email) => {
    if (email) address.value = email
  },
)
async function resend() {
  if (busy.value) return
  busy.value = true
  message.value = ''
  try {
    const result = await $fetch<{ message: string }>(
      '/api/auth/resend-verification',
      { method: 'POST', body: { email: address.value }, retry: false },
    )
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
    <div>
      <input
        :id="id"
        v-model="address"
        name="email"
        type="email"
        required
        maxlength="254"
        autocomplete="email"
        placeholder="注册邮箱"
        :disabled="busy"
      />
      <button :disabled="busy">
        <UiBusyLabel :busy="busy">发送验证邮件</UiBusyLabel>
      </button>
    </div>
    <p v-if="message" role="status">{{ message }}</p>
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
.resend-verification > div {
  display: flex;
  gap: 8px;
}
.resend-verification input {
  min-width: 0;
  flex: 1;
  background: var(--bg-input);
  color: var(--text-main);
  border: 1px solid var(--border-line);
  border-radius: 7px;
  padding: 10px;
  font: inherit;
  font-size: 12px;
}
.resend-verification button {
  padding: 10px;
  border: 1px solid var(--border-line);
  border-radius: 7px;
  color: var(--text-main);
  background: var(--bg-card);
  font-size: 12px;
  white-space: nowrap;
  cursor: pointer;
}
.resend-verification button:disabled {
  opacity: 0.5;
}
.resend-verification p {
  font-size: 12px;
  line-height: 1.7;
  color: var(--text-muted);
  margin-top: 10px;
}
</style>
