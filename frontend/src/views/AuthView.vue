<template>
  <div class="min-h-screen flex flex-col justify-center items-center px-4 py-12">
    <div class="w-full max-w-[380px]">
      <div class="flex flex-col items-center mb-6 text-center">
        <img src="/favicon.ico" alt="AttackBox Logo" class="w-16 h-16 rounded-2xl object-contain mb-3" />
        <h1 class="text-base font-semibold tracking-tight text-zinc-900 font-mono">AttackBox</h1>
        <p class="text-xs text-zinc-500 mt-1 font-mono">OFFENSIVE SECURITY WORKSPACE</p>
      </div>

      <div class="shadcn-card p-6">
        <div v-if="allowRegister" class="grid grid-cols-2 p-1 bg-zinc-100 rounded-lg mb-5">
          <button
            type="button"
            @click="authMode = 'login'"
            :class="[
              'py-1.5 text-xs font-medium rounded-md transition-all',
              authMode === 'login'
                ? 'bg-white text-zinc-900 shadow-sm'
                : 'text-zinc-500 hover:text-zinc-900'
            ]"
          >
            登录
          </button>
          <button
            type="button"
            @click="authMode = 'register'"
            :class="[
              'py-1.5 text-xs font-medium rounded-md transition-all',
              authMode === 'register'
                ? 'bg-white text-zinc-900 shadow-sm'
                : 'text-zinc-500 hover:text-zinc-900'
            ]"
          >
            注册
          </button>
        </div>
        <div v-else class="text-center font-medium text-xs text-zinc-700 mb-5 pb-1 border-b border-zinc-100">
          账号登录
        </div>

        <form @submit.prevent="handleSubmit" class="space-y-4">
          <div>
            <label class="block text-xs font-medium text-zinc-700 mb-1.5">账号</label>
            <input
              v-model.trim="username"
              type="text"
              required
              placeholder="3-32位字母数字下划线"
              class="w-full px-3 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 transition-shadow"
            />
          </div>
          <div>
            <label class="block text-xs font-medium text-zinc-700 mb-1.5">密码</label>
            <input
              v-model="password"
              type="password"
              required
              minlength="6"
              placeholder="•••••••• (至少 6 位)"
              class="w-full px-3 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 transition-shadow"
            />
          </div>

          <!-- 注册专用字段: 邮箱、验证码、AFF 邀请码 -->
          <template v-if="authMode === 'register'">
            <div>
              <label class="block text-xs font-medium text-zinc-700 mb-1.5">
                安全邮箱 <span class="text-[10px] text-zinc-400 font-normal">(仅支持 QQ / Gmail / Outlook)</span>
              </label>
              <input
                v-model.trim="email"
                type="email"
                required
                placeholder="user@qq.com / user@gmail.com / ..."
                class="w-full px-3 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 transition-shadow"
              />
            </div>

            <div>
              <label class="block text-xs font-medium text-zinc-700 mb-1.5">邮箱验证码</label>
              <div class="flex items-center gap-2">
                <input
                  v-model.trim="verificationCode"
                  type="text"
                  required
                  maxlength="6"
                  placeholder="6位验证码"
                  class="flex-1 px-3 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 transition-shadow font-mono tracking-widest text-center"
                />
                <button
                  type="button"
                  @click="handleSendCode"
                  :disabled="sendCooldown > 0 || sendingCode"
                  class="px-3 py-2 text-xs font-medium rounded-lg border border-zinc-200 text-zinc-700 hover:bg-zinc-50 disabled:opacity-50 transition-all cursor-pointer whitespace-nowrap shadow-xs"
                >
                  <span v-if="sendingCode">发送中...</span>
                  <span v-else-if="sendCooldown > 0">{{ sendCooldown }}s 后重试</span>
                  <span v-else>获取验证码</span>
                </button>
              </div>
            </div>

            <div>
              <label class="block text-xs font-medium text-zinc-700 mb-1.5">
                AFF 邀请码 <span class="text-[10px] text-zinc-400 font-normal">(选填)</span>
              </label>
              <input
                v-model.trim="affCode"
                type="text"
                maxlength="8"
                placeholder="8位专属邀请码 (可选)"
                class="w-full px-3 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 transition-shadow font-mono uppercase"
              />
            </div>
          </template>

          <button
            type="submit"
            :disabled="loading"
            class="w-full py-2 px-4 rounded-lg text-xs font-medium shadcn-btn-primary flex items-center justify-center gap-1.5 disabled:opacity-50 cursor-pointer"
          >
            <span v-if="loading">处理中...</span>
            <span v-else>{{ authMode === 'login' ? '进入平台' : '注册并登录' }}</span>
          </button>
        </form>

        <!-- 第三方快捷授权入口 -->
        <div v-if="oauthProviders.length > 0" class="mt-5">
          <div class="relative flex items-center justify-center my-4">
            <div class="border-t border-zinc-200 w-full"></div>
            <span class="bg-white px-2.5 text-[11px] font-mono text-zinc-400 uppercase tracking-wider absolute">OR</span>
          </div>

          <div class="grid grid-cols-1 gap-2">
            <button
              v-if="oauthProviders.includes('github')"
              type="button"
              @click="startOAuth('github')"
              class="w-full py-2 px-3 bg-zinc-900 hover:bg-zinc-800 text-white rounded-lg text-xs font-medium flex items-center justify-center gap-2 transition-all shadow-sm cursor-pointer"
            >
              <svg class="w-4 h-4 fill-current" viewBox="0 0 24 24">
                <path fill-rule="evenodd" clip-rule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"></path>
              </svg>
              <span>使用 GitHub 继续</span>
            </button>

            <button
              v-if="oauthProviders.includes('outlook')"
              type="button"
              @click="startOAuth('outlook')"
              class="w-full py-2 px-3 bg-white hover:bg-zinc-50 border border-zinc-200 text-zinc-800 rounded-lg text-xs font-medium flex items-center justify-center gap-2 transition-all shadow-sm cursor-pointer"
            >
              <svg class="w-4 h-4" viewBox="0 0 21 21">
                <rect x="1" y="1" width="9" height="9" fill="#f25022"/>
                <rect x="11" y="1" width="9" height="9" fill="#7fba00"/>
                <rect x="1" y="11" width="9" height="9" fill="#00a4ef"/>
                <rect x="11" y="11" width="9" height="9" fill="#ffb900"/>
              </svg>
              <span>使用 Outlook / Microsoft 继续</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 两步建号/确认弹窗 (OAuth Setup Modal) -->
    <div v-if="showOAuthModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-xs px-4">
      <div class="bg-white rounded-xl shadow-2xl border border-zinc-200 w-full max-w-sm p-6 relative">
        <div class="flex items-center gap-2.5 mb-4">
          <div class="w-8 h-8 rounded-full bg-emerald-50 text-emerald-600 flex items-center justify-center text-xs font-bold">
            ✓
          </div>
          <div>
            <h3 class="text-sm font-semibold text-zinc-900">第三方授权成功</h3>
            <p class="text-xs text-zinc-500">确认或自定义您的 AttackBox 账号</p>
          </div>
        </div>

        <form @submit.prevent="handleCompleteOAuth" class="space-y-4">
          <div>
            <label class="block text-xs font-mono font-medium text-zinc-700 mb-1.5">ATTACKBOX 账号</label>
            <input
              v-model.trim="oauthUsername"
              type="text"
              required
              placeholder="3-32位字母、数字或连字符"
              class="w-full px-3 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono"
            />
          </div>

          <div>
            <label class="block text-xs font-mono font-medium text-zinc-700 mb-1.5">
              设置登录密码 <span class="text-zinc-500 font-normal font-sans">(至少 6 位，用于密码登入与安全凭据修改)</span>
            </label>
            <input
              v-model="oauthPassword"
              type="password"
              required
              minlength="6"
              placeholder="••••••••"
              class="w-full px-3 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900"
            />
          </div>

          <div class="flex items-center gap-2 pt-2">
            <button
              type="button"
              @click="cancelOAuthModal"
              class="flex-1 py-2 px-3 text-xs font-medium rounded-lg border border-zinc-200 text-zinc-600 hover:bg-zinc-50 transition-all cursor-pointer"
            >
              取消
            </button>
            <button
              type="submit"
              :disabled="oauthLoading"
              class="flex-1 py-2 px-3 text-xs font-medium rounded-lg bg-zinc-900 text-white hover:bg-zinc-800 disabled:opacity-50 transition-all cursor-pointer"
            >
              <span v-if="oauthLoading">提交中...</span>
              <span v-else>确认建号并登入</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const emit = defineEmits(['auth-success', 'show-toast'])

const authMode = ref('login')
const username = ref('')
const password = ref('')
const email = ref('')
const verificationCode = ref('')
const affCode = ref('')

const loading = ref(false)
const allowRegister = ref(true)
const oauthProviders = ref([])

const sendingCode = ref(false)
const sendCooldown = ref(0)
let cooldownTimer = null

// 两步建号弹窗状态
const showOAuthModal = ref(false)
const oauthTicket = ref('')
const oauthUsername = ref('')
const oauthPassword = ref('')
const oauthLoading = ref(false)

function isValidEmail(val) {
  if (!val) return false
  const allowedSuffixes = [
    '@qq.com', '@foxmail.com',
    '@gmail.com', '@googlemail.com',
    '@outlook.com', '@hotmail.com', '@live.com', '@msn.com',
  ]
  const lower = val.trim().toLowerCase()
  return allowedSuffixes.some(s => lower.endsWith(s))
}

async function handleSendCode() {
  if (!email.value) {
    emit('show-toast', '请先输入注册邮箱')
    return
  }
  if (!isValidEmail(email.value)) {
    emit('show-toast', '仅支持 QQ 邮箱、谷歌 Gmail 及微软 Outlook 邮箱')
    return
  }

  sendingCode.value = true
  try {
    const resp = await fetch('/api/auth/send-verification-code', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email.value }),
    })
    const data = await resp.json()
    if (!resp.ok || !data.success) {
      emit('show-toast', data.error || '验证码发送失败')
      return
    }
    emit('show-toast', '验证码已发送，请查收邮件 (10分钟内有效)')
    sendCooldown.value = 60
    if (cooldownTimer) clearInterval(cooldownTimer)
    cooldownTimer = setInterval(() => {
      sendCooldown.value--
      if (sendCooldown.value <= 0) {
        clearInterval(cooldownTimer)
      }
    }, 1000)
  } catch (err) {
    emit('show-toast', '发送验证码请求异常: ' + err.message)
  } finally {
    sendingCode.value = false
  }
}

onMounted(async () => {
  // 1. 检查系统策略
  try {
    const res = await fetch('/api/public/config')
    if (res.ok) {
      const data = await res.json()
      if (typeof data.allowRegister === 'boolean') {
        allowRegister.value = data.allowRegister
        if (!allowRegister.value && authMode.value === 'register') {
          authMode.value = 'login'
        }
      }
    }
  } catch (e) {}

  // 2. 加载已启用的第三方登录
  try {
    const res = await fetch('/api/public/oauth-providers')
    if (res.ok) {
      const data = await res.json()
      if (Array.isArray(data.providers)) {
        oauthProviders.value = data.providers
      }
    }
  } catch (e) {}

  // 3. 处理 URL 参数中的第三方错误提示、两步注册 Ticket 或 AFF 邀请码
  const urlParams = new URLSearchParams(window.location.search)
  const errParam = urlParams.get('error')
  if (errParam) {
    emit('show-toast', '第三方登录失败: ' + errParam)
    window.history.replaceState(null, '', window.location.pathname)
  }

  const affParam = urlParams.get('aff')
  if (affParam) {
    affCode.value = affParam.trim().toUpperCase()
    if (allowRegister.value) {
      authMode.value = 'register'
    }
  }

  const ticketParam = urlParams.get('oauth_ticket')
  if (ticketParam) {
    oauthTicket.value = ticketParam
    showOAuthModal.value = true
    // 默认生成一个随机短账号名备选
    oauthUsername.value = 'box_' + Math.floor(Math.random() * 8999 + 1000)
  }
})

function startOAuth(provider) {
  window.location.href = `/api/oauth/${provider}/authorize`
}

function cancelOAuthModal() {
  showOAuthModal.value = false
  oauthTicket.value = ''
  window.history.replaceState(null, '', window.location.pathname)
}

async function handleCompleteOAuth() {
  if (!oauthUsername.value) return
  if (!oauthPassword.value || oauthPassword.value.length < 6) {
    emit('show-toast', '请设置至少 6 位的登录密码')
    return
  }
  oauthLoading.value = true
  try {
    const resp = await fetch('/api/oauth/complete-registration', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ticket: oauthTicket.value,
        username: oauthUsername.value,
        password: oauthPassword.value,
      }),
    })
    const data = await resp.json()
    if (!resp.ok || !data.success) {
      emit('show-toast', data.error || '创建账号失败')
      return
    }
    showOAuthModal.value = false
    window.history.replaceState(null, '', '/workspace')
    emit('show-toast', '欢迎加入 AttackBox，账号创建成功！')
    emit('auth-success', data.user)
  } catch (err) {
    emit('show-toast', '请求失败: ' + err.message)
  } finally {
    oauthLoading.value = false
  }
}

async function handleSubmit() {
  if (!username.value || !password.value) return
  if (!allowRegister.value && authMode.value === 'register') {
    authMode.value = 'login'
  }

  const isRegister = authMode.value === 'register'
  if (isRegister) {
    if (!email.value) {
      emit('show-toast', '请输入注册安全邮箱')
      return
    }
    if (!isValidEmail(email.value)) {
      emit('show-toast', '仅支持 QQ 邮箱、谷歌 Gmail 及微软 Outlook 邮箱')
      return
    }
    if (!verificationCode.value) {
      emit('show-toast', '请输入收到的 6 位邮箱验证码')
      return
    }
  }

  loading.value = true
  try {
    const endpoint = isRegister ? '/api/register' : '/api/login'
    const payload = isRegister ? {
      username: username.value,
      password: password.value,
      email: email.value,
      code: verificationCode.value,
      affCode: affCode.value,
    } : {
      username: username.value,
      password: password.value,
    }

    const resp = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    const data = await resp.json()
    if (!resp.ok || !data.success) {
      emit('show-toast', data.error || (isRegister ? '注册失败' : '登录失败，请检查账号密码'))
      return
    }
    emit('show-toast', isRegister ? '注册成功并登入' : '登录成功')
    emit('auth-success', data.user)
  } catch (err) {
    emit('show-toast', '请求失败: ' + err.message)
  } finally {
    loading.value = false
  }
}
</script>
