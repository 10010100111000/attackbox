<template>
  <section class="max-w-xl mx-auto space-y-6">
    <!-- 1. 账号基本资料卡片 -->
    <div class="shadcn-card p-5 space-y-3">
      <div class="pb-3 border-b border-zinc-100 flex items-center justify-between">
        <div>
          <h2 class="text-xs font-mono font-semibold text-zinc-900 tracking-wider">ACCOUNT PROFILE</h2>
          <p class="text-[11px] text-zinc-500 mt-0.5">当前登录账号的基础安全身份与授权属性</p>
        </div>
        <span class="px-2 py-0.5 rounded text-[10px] font-mono font-semibold uppercase" :class="user.role === 'admin' ? 'bg-amber-100 text-amber-900' : 'bg-zinc-100 text-zinc-700'">
          {{ user.role || 'user' }}
        </span>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs pt-1">
        <div>
          <span class="block text-[11px] font-mono text-zinc-400">USERNAME</span>
          <span class="font-mono font-medium text-zinc-800">{{ user.username || '-' }}</span>
        </div>
        <div>
          <span class="block text-[11px] font-mono text-zinc-400">BOUND EMAIL</span>
          <span class="font-mono font-medium text-zinc-800">{{ user.email || '未绑定' }}</span>
        </div>
      </div>
    </div>

    <!-- 2. AFF 专属邀请推广卡片 -->
    <div class="shadcn-card p-5 space-y-4">
      <div class="pb-3 border-b border-zinc-100 flex items-center justify-between">
        <div>
          <h2 class="text-xs font-mono font-semibold text-zinc-900 tracking-wider">AFFILIATE PROGRAM</h2>
          <p class="text-[11px] text-zinc-500 mt-0.5">邀请好友或团队成员注册，专属 8 位推广邀请码</p>
        </div>
        <span class="px-2 py-0.5 rounded text-[10px] font-mono font-bold bg-zinc-900 text-white tracking-widest">
          {{ user.affCode || 'ATTACK01' }}
        </span>
      </div>

      <div class="space-y-3 text-xs">
        <div>
          <label class="block font-medium text-zinc-700 mb-1 font-mono">专属推广邀请链接</label>
          <div class="flex items-center gap-2">
            <input
              type="text"
              readonly
              :value="referralLink"
              class="flex-1 px-3 py-2 border border-zinc-200 rounded-lg bg-zinc-50 font-mono text-xs text-zinc-700 select-all focus:outline-none"
            />
            <button
              type="button"
              @click="copyReferralLink"
              class="px-3 py-2 rounded-lg text-xs font-medium bg-zinc-900 text-white hover:bg-zinc-800 transition-all cursor-pointer whitespace-nowrap shadow-xs"
            >
              复制链接
            </button>
            <button
              type="button"
              @click="copyAffCode"
              class="px-3 py-2 rounded-lg text-xs font-medium border border-zinc-200 text-zinc-700 hover:bg-zinc-50 transition-all cursor-pointer whitespace-nowrap shadow-xs"
            >
              复制邀请码
            </button>
          </div>
          <p class="text-[11px] text-zinc-400 mt-1.5">
            受邀用户点击链接访问注册页时将自动绑定您的专属邀请码。
          </p>
        </div>
      </div>
    </div>

    <!-- 3. 修改密码卡片 -->
    <div class="shadcn-card p-5">
      <div class="pb-4 border-b border-zinc-100">
        <h2 class="text-xs font-mono font-semibold text-zinc-900 tracking-wider">UPDATE PASSWORD</h2>
        <p class="text-xs text-zinc-500 mt-0.5">修改当前登录账户的安全凭证（至少 6 位）</p>
      </div>

      <form @submit.prevent="handleChangePassword" class="space-y-4 mt-4">
        <div>
          <label class="block text-xs font-medium text-zinc-700 mb-1.5 font-mono">CURRENT PASSWORD</label>
          <input
            v-model="oldPassword"
            type="password"
            required
            placeholder="输入当前使用的旧密码"
            class="w-full px-3 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 transition-shadow font-mono"
          />
        </div>

        <div>
          <label class="block text-xs font-medium text-zinc-700 mb-1.5 font-mono">NEW PASSWORD</label>
          <input
            v-model="newPassword"
            type="password"
            required
            minlength="6"
            placeholder="输入新密码（至少 6 位）"
            class="w-full px-3 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 transition-shadow font-mono"
          />
        </div>

        <div>
          <label class="block text-xs font-medium text-zinc-700 mb-1.5 font-mono">CONFIRM NEW PASSWORD</label>
          <input
            v-model="confirmPassword"
            type="password"
            required
            minlength="6"
            placeholder="再次输入新密码"
            class="w-full px-3 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 transition-shadow font-mono"
          />
        </div>

        <div class="pt-2 flex justify-end">
          <button
            type="submit"
            :disabled="loading"
            class="px-4 py-2 rounded-lg text-xs font-semibold bg-zinc-900 text-white hover:bg-zinc-800 shadow-sm transition-all disabled:opacity-50 cursor-pointer"
          >
            {{ loading ? 'UPDATING...' : 'UPDATE PASSWORD' }}
          </button>
        </div>
      </form>
    </div>
  </section>
</template>

<script setup>
import { ref, computed } from 'vue'
import { api } from '../api.js'

const props = defineProps({
  user: {
    type: Object,
    default: () => ({}),
  },
})

const emit = defineEmits(['show-toast'])

const referralLink = computed(() => {
  const code = props.user?.affCode || ''
  return `${window.location.origin}/login?aff=${code}`
})

async function copyReferralLink() {
  try {
    await navigator.clipboard.writeText(referralLink.value)
    emit('show-toast', '专属推广链接已复制到剪贴板')
  } catch (e) {
    emit('show-toast', '复制失败，请手动选择复制')
  }
}

async function copyAffCode() {
  const code = props.user?.affCode || ''
  if (!code) return
  try {
    await navigator.clipboard.writeText(code)
    emit('show-toast', `专属邀请码 ${code} 已复制`)
  } catch (e) {
    emit('show-toast', '复制失败，请手动复制')
  }
}

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const loading = ref(false)

async function handleChangePassword() {
  if (!oldPassword.value || !newPassword.value) return
  if (newPassword.value.length < 6) {
    emit('show-toast', '新密码长度至少 6 位')
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    emit('show-toast', '两次输入的新密码不一致')
    return
  }

  loading.value = true
  try {
    const res = await api.changePassword(oldPassword.value, newPassword.value)
    if (!res.success) {
      emit('show-toast', res.error || '修改密码失败')
      return
    }
    emit('show-toast', '安全凭据已成功更新')
    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
  } catch (e) {
    emit('show-toast', e.message || '网络请求异常')
  } finally {
    loading.value = false
  }
}
</script>
