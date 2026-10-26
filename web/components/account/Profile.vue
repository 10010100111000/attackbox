<script setup lang="ts">
import { Check, KeyRound, Mail, UserRound } from 'lucide-vue-next'
import { onBeforeRouteLeave } from 'vue-router'

const { currentUser } = useAuth()
const name = ref(currentUser.value?.displayName || '')
const busy = ref(false)
const message = ref('')
const error = ref('')
const dirty = computed(
  () => name.value.trim() !== currentUser.value?.displayName,
)
const invalid = computed(
  () => !name.value.trim() || name.value.trim().length > 64,
)
watch(
  name,
  () => {
    message.value = ''
    error.value = ''
  },
  { flush: 'sync' },
)
function reset() {
  name.value = currentUser.value?.displayName || ''
  message.value = ''
  error.value = ''
}
onBeforeRouteLeave(
  () => !dirty.value || window.confirm('昵称修改尚未保存，确认离开？'),
)
async function save() {
  if (busy.value || invalid.value || !dirty.value || !currentUser.value) return
  const owner = currentUser.value.id
  busy.value = true
  message.value = ''
  error.value = ''
  try {
    const result = await $fetch<{ saved: true; displayName: string }>(
      '/api/me/profile',
      { method: 'PUT', body: { displayName: name.value }, retry: false },
    )
    if (currentUser.value?.id !== owner) return
    currentUser.value = {
      ...currentUser.value,
      displayName: result.displayName,
    }
    name.value = result.displayName
    message.value = '资料已保存'
  } catch (e: any) {
    if (currentUser.value?.id === owner)
      error.value = e?.data?.message || '保存失败，修改已保留，请重试。'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section aria-labelledby="personal-profile-title">
    <header class="personal-section-heading">
      <div>
        <span class="personal-eyebrow">ACCOUNT</span>
        <h1 id="personal-profile-title">账户资料</h1>
      </div>
    </header>
    <form class="personal-settings" @submit.prevent="save">
      <div class="personal-settings-heading">
        <UserRound :size="18" aria-hidden="true" />
        <h2>基本资料</h2>
      </div>
      <div class="personal-field">
        <label for="personal-nickname">昵称</label>
        <div>
          <input
            id="personal-nickname"
            v-model="name"
            :disabled="busy"
            :aria-invalid="invalid"
            :aria-describedby="
              invalid ? 'personal-name-error' : 'personal-name-hint'
            "
            maxlength="64"
            required
            autocomplete="nickname"
          />
          <p
            v-if="invalid"
            id="personal-name-error"
            class="personal-field-error"
          >
            昵称需为 1–64 个字符。
          </p>
          <p v-else id="personal-name-hint" class="personal-field-hint">
            用于个人中心与头像菜单，最多 64 个字符。
          </p>
        </div>
      </div>
      <div class="personal-field">
        <span class="personal-field-label">邮箱</span>
        <div>
          <span class="personal-email">
            <Mail :size="15" aria-hidden="true" />
            {{ currentUser?.email }}
          </span>
          <p class="personal-field-hint">当前登录邮箱</p>
        </div>
      </div>
      <div class="personal-save-row">
        <p v-if="message" role="status" class="personal-save-success">
          <Check :size="15" aria-hidden="true" />
          {{ message }}
        </p>
        <p v-else-if="error" role="alert" class="personal-field-error">
          {{ error }}
        </p>
        <span v-else class="personal-save-hint">
          {{ dirty ? '有尚未保存的修改' : '资料已同步' }}
        </span>
        <div class="personal-save-actions">
          <button
            v-if="dirty"
            type="button"
            class="personal-button secondary"
            :disabled="busy"
            @click="reset"
          >
            取消修改
          </button>
          <button class="personal-button" :disabled="busy || invalid || !dirty">
            {{ busy ? '正在保存…' : '保存资料' }}
          </button>
        </div>
      </div>
    </form>
    <section
      class="personal-security"
      aria-labelledby="personal-security-title"
    >
      <div>
        <KeyRound :size="18" aria-hidden="true" />
        <div>
          <h2 id="personal-security-title">登录与安全</h2>
          <p>使用邮箱和密码登录</p>
        </div>
      </div>
      <NuxtLink to="/auth?mode=recover" class="personal-text-link">
        找回密码
      </NuxtLink>
    </section>
  </section>
</template>
