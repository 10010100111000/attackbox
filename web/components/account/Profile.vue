<script setup lang="ts">
import { Check, Mail, UserRound, ShieldCheck } from 'lucide-vue-next'
import { onBeforeRouteLeave } from 'vue-router'
import { apiErrorMessage, apiFetch } from '~/utils/api'
const confirmAction = useConfirm()

const props = withDefaults(defineProps<{ active?: boolean }>(), { active: true })
const { currentUser } = useAuth()
const owner = computed(() => currentUser.value?.id || '')
const {
  data: accountResponse,
  pending: accountPending,
  error: accountError,
  execute: loadAccount
} = useLazyFetch<{ data: { id: string; hasPassword: boolean } }>('/api/me/profile', {
  key: 'account:profile',
  immediate: props.active && !!owner.value,
  watch: false
})
const account = computed(() =>
  accountResponse.value?.data.id === owner.value ? accountResponse.value.data : null
)
const passwordOpen = ref(false),
  passwordTrigger = ref<HTMLButtonElement | null>(null)
watch([() => props.active, owner], ([active]) => {
  passwordOpen.value = false
  if (active && owner.value) void loadAccount({ dedupe: 'cancel' })
})
async function closePassword() {
  passwordOpen.value = false
  await nextTick()
  passwordTrigger.value?.focus()
}

const name = ref(currentUser.value?.displayName || '')
const busy = ref(false)
const message = ref('')
const error = ref('')
const dirty = computed(() => name.value.trim() !== currentUser.value?.displayName)
const invalid = computed(() => !name.value.trim() || name.value.trim().length > 64)
watch(
  name,
  () => {
    message.value = ''
    error.value = ''
  },
  { flush: 'sync' }
)
function reset() {
  name.value = currentUser.value?.displayName || ''
  message.value = ''
  error.value = ''
}
onBeforeRouteLeave(
  () =>
    !dirty.value ||
    confirmAction({
      title: '放弃昵称修改？',
      description: '未保存的昵称修改将丢失。',
      confirmLabel: '放弃修改',
      cancelLabel: '继续编辑'
    })
)
async function save() {
  if (busy.value || invalid.value || !dirty.value || !currentUser.value) return
  const owner = currentUser.value.id
  busy.value = true
  message.value = ''
  error.value = ''
  try {
    const result = await apiFetch<{ saved: true; displayName: string }>('/api/me/profile', {
      method: 'PUT',
      body: { displayName: name.value },
      retry: false
    })
    if (currentUser.value?.id !== owner) return
    currentUser.value = {
      ...currentUser.value,
      displayName: result.displayName
    }
    name.value = result.displayName
    message.value = '资料已保存'
  } catch (e) {
    if (currentUser.value?.id === owner)
      error.value = apiErrorMessage(e, '保存失败，修改已保留，请重试。')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section aria-labelledby="personal-profile-title">
    <header class="personal-section-heading">
      <div>
        <h1 id="personal-profile-title">账户资料</h1>
      </div>
    </header>
    <div class="personal-settings">
      <form class="profile-section" @submit.prevent="save">
        <div class="personal-settings-heading">
          <UserRound :size="18" aria-hidden="true" />
          <h2>个人信息</h2>
        </div>
        <div class="personal-field">
          <label for="personal-nickname">昵称</label>
          <div>
            <input
              id="personal-nickname"
              v-model="name"
              :disabled="busy"
              :aria-invalid="invalid"
              :aria-describedby="invalid ? 'personal-name-error' : undefined"
              maxlength="64"
              required
              autocomplete="nickname"
            />
            <p v-if="invalid" id="personal-name-error" class="personal-field-error">
              昵称需为 1–64 个字符。
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
            {{ dirty ? '有未保存的修改' : '' }}
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
              <UiBusyLabel :busy="busy">保存资料</UiBusyLabel>
            </button>
          </div>
        </div>
      </form>
      <section class="profile-section account-security" aria-labelledby="account-security-title">
        <div class="personal-settings-heading">
          <ShieldCheck :size="18" aria-hidden="true" />
          <h2 id="account-security-title">登录与安全</h2>
        </div>
        <UiBusyIndicator
          v-if="accountPending && !account"
          class="account-security-state"
          label="登录设置加载"
        />
        <div v-else-if="accountError || !account" class="account-security-state">
          <p role="alert">登录设置暂时无法读取。</p>
          <button type="button" class="personal-button secondary" @click="loadAccount()">
            重试
          </button>
        </div>
        <template v-else>
          <div class="personal-field account-password-row">
            <span class="personal-field-label">本地密码</span>
            <div class="account-login-method">
              <span>{{ account.hasPassword ? '已设置' : '未设置' }}</span
              ><button
                v-if="account.hasPassword && !passwordOpen"
                ref="passwordTrigger"
                type="button"
                class="personal-button secondary"
                :aria-expanded="passwordOpen"
                aria-controls="account-password-editor"
                @click="passwordOpen = true"
              >
                修改密码
              </button>
            </div>
          </div>
          <AccountPassword
            v-if="active && account.hasPassword && passwordOpen"
            id="account-password-editor"
            @close="closePassword"
          />
        </template>
      </section>
    </div>
  </section>
</template>

<style scoped>
.profile-section + .profile-section {
  border-top: 1px solid var(--line);
}
.account-login-method {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  min-height: 38px;
  font-size: 13px;
}
.account-password-row .personal-field-label {
  padding-top: 10px;
}
.account-security-state {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 24px 28px;
  font-size: 12px;
  color: var(--muted);
}
</style>
