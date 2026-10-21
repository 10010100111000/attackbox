<template>
  <div class="min-h-screen flex flex-col items-center px-4 pt-16 sm:pt-24 pb-16">
    <div class="w-full max-w-[400px]">
      <!-- 平台标志与副标题 -->
      <div class="flex flex-col items-center mb-6 text-center">
        <img src="/favicon.ico" alt="AttackBox Logo" class="w-14 h-14 rounded-2xl object-contain mb-2.5 shadow-xs" />
        <h1 class="text-base font-semibold tracking-tight text-zinc-900 font-mono">AttackBox</h1>
        <p class="text-[11px] text-zinc-500 mt-0.5 font-mono tracking-wider">OFFENSIVE SECURITY WORKSPACE</p>
      </div>

      <!-- 核心卡片 -->
      <div class="shadcn-card p-6 shadow-sm border border-zinc-200/80 rounded-xl transition-all duration-200">
        <!-- 登录 / 注册 Tab 切换 -->
        <div v-if="allowRegister" class="grid grid-cols-2 p-1 bg-zinc-100 rounded-lg mb-5">
          <button
            type="button"
            @click="switchAuthMode('login')"
            :class="[
              'py-1.5 text-xs font-medium rounded-md transition-all cursor-pointer',
              authMode === 'login'
                ? 'bg-white text-zinc-900 shadow-xs'
                : 'text-zinc-500 hover:text-zinc-900'
            ]"
          >
            登录
          </button>
          <button
            type="button"
            @click="switchAuthMode('register')"
            :class="[
              'py-1.5 text-xs font-medium rounded-md transition-all cursor-pointer',
              authMode === 'register'
                ? 'bg-white text-zinc-900 shadow-xs'
                : 'text-zinc-500 hover:text-zinc-900'
            ]"
          >
            注册
          </button>
        </div>
        <div v-else class="text-center font-medium text-xs text-zinc-700 mb-5 pb-1 border-b border-zinc-100">
          安全登录
        </div>

        <!-- ================= 登录表单: 统一安全邮箱登录 ================= -->
        <form v-show="authMode === 'login'" @submit.prevent="handleLogin" class="space-y-4">
          <!-- 登录邮箱 -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <label class="text-xs font-medium text-zinc-700">安全邮箱</label>
            </div>
            <div class="relative">
              <input
                v-model.trim="loginEmail"
                type="email"
                required
                autocomplete="email"
                placeholder="name@example.com"
                class="w-full pl-3 pr-8 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 focus:border-transparent transition-all"
              />
              <span v-if="loginEmail && isValidEmail(loginEmail)" class="absolute right-2.5 top-2.5 text-emerald-500 text-xs">
                ✓
              </span>
            </div>
          </div>

          <!-- 登录密码 -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <label class="text-xs font-medium text-zinc-700">登录密码</label>
            </div>
            <div class="relative">
              <input
                v-model="loginPassword"
                :type="showLoginPassword ? 'text' : 'password'"
                required
                minlength="6"
                autocomplete="current-password"
                placeholder="输入登录密码 (至少 6 位)"
                :class="[
                  'w-full pl-3 pr-10 py-2 text-xs bg-white border rounded-lg focus:outline-none focus:ring-2 transition-all font-mono',
                  loginPassword && loginPassword.length < 6
                    ? 'border-amber-300 focus:ring-amber-500'
                    : 'border-zinc-200 focus:ring-zinc-900 focus:border-transparent'
                ]"
              />
              <button
                type="button"
                @click="showLoginPassword = !showLoginPassword"
                tabindex="-1"
                class="absolute right-2.5 top-1/2 -translate-y-1/2 flex items-center justify-center text-zinc-400 hover:text-zinc-700 transition-colors p-1 cursor-pointer focus:outline-none"
                :title="showLoginPassword ? '隐藏密码明文' : '显示密码明文'"
              >
                <!-- Eye / Eye-off 图标 -->
                <svg v-if="!showLoginPassword" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z"></path>
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                </svg>
                <svg v-else class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M3.98 8.223A10.477 10.477 0 001.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.451 10.451 0 0112 4.5c4.756 0 8.773 3.162 10.065 7.498a10.522 10.522 0 01-4.293 5.774M6.228 6.228L3 3m3.228 3.228l3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 10-4.243-4.243m4.242 4.242L9.88 9.88"></path>
                </svg>
              </button>
            </div>
            <p v-if="loginPassword && loginPassword.length < 6" class="text-[10px] text-amber-600 mt-1">
              密码需至少 6 位 (当前 {{ loginPassword.length }} 位)
            </p>
          </div>

          <button
            type="submit"
            :disabled="!isLoginValid"
            :class="[
              'w-full py-2.5 px-4 rounded-lg text-xs font-medium transition-all shadow-xs mt-3 flex items-center justify-center gap-1.5',
              isLoginValid
                ? 'bg-zinc-900 text-white hover:bg-zinc-800 cursor-pointer active:scale-[0.99]'
                : 'bg-zinc-100 text-zinc-400 cursor-not-allowed border border-zinc-200/80 shadow-none'
            ]"
          >
            <svg v-if="loading" class="animate-spin h-3.5 w-3.5 text-zinc-400" viewBox="0 0 24 24" fill="none">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
            </svg>
            <span>{{ loading ? '验证登入中...' : '进入平台' }}</span>
          </button>
        </form>

        <!-- ================= 注册表单: 纯安全邮箱注册流程 ================= -->
        <form v-show="authMode === 'register'" @submit.prevent="handleRegister" class="space-y-4">
          <!-- 注册邮箱 -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <label class="text-xs font-medium text-zinc-700">安全邮箱</label>
              <span class="text-[10px] text-zinc-400 font-normal">支持 QQ / Gmail / Outlook</span>
            </div>
            <div class="relative">
              <input
                v-model.trim="regEmail"
                type="email"
                required
                autocomplete="email"
                placeholder="your-name@qq.com"
                class="w-full pl-3 pr-8 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 focus:border-transparent transition-all"
              />
              <span v-if="regEmail && isValidEmail(regEmail)" class="absolute right-2.5 top-2.5 text-emerald-500 text-xs">
                ✓
              </span>
            </div>
            <!-- 不支持的域名实时温和提示 -->
            <p v-if="regEmail && regEmail.includes('@') && !isValidEmail(regEmail)" class="text-[10px] text-amber-600 mt-1">
              仅支持 QQ 邮箱 (@qq.com)、谷歌 Gmail (@gmail.com) 或微软 Outlook 邮箱
            </p>
          </div>

          <!-- 邮箱验证码 (无冗余 placeholder，输入框与倒计时按钮协同整合) -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <label class="text-xs font-medium text-zinc-700">邮箱验证码</label>
              <span v-if="codeSentNotice" class="text-[10px] font-medium text-emerald-600 flex items-center gap-1 animate-pulse">
                <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
                验证码已发送至收件箱
              </span>
            </div>

            <div class="relative flex items-center">
              <input
                v-model.trim="verificationCode"
                type="text"
                inputmode="numeric"
                pattern="[0-9]*"
                maxlength="6"
                autocomplete="one-time-code"
                @input="handleCodeInput"
                class="w-full pl-3 pr-28 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 focus:border-transparent transition-all font-mono tracking-widest text-left"
              />

              <!-- 获取 / 重发验证码按钮 -->
              <div class="absolute right-1.5 flex items-center">
                <button
                  type="button"
                  @click="handleSendCode"
                  :disabled="!canSendCode"
                  :class="[
                    'px-2.5 py-1 text-xs font-medium rounded-md transition-all whitespace-nowrap shadow-2xs',
                    canSendCode
                      ? 'text-zinc-700 bg-zinc-100 hover:bg-zinc-200 hover:text-zinc-900 cursor-pointer'
                      : 'text-zinc-400 bg-zinc-50 border border-zinc-200/50 cursor-not-allowed opacity-60'
                  ]"
                >
                  <span v-if="sendingCode" class="flex items-center gap-1 text-[11px]">
                    <svg class="animate-spin h-3 w-3 text-zinc-600" viewBox="0 0 24 24" fill="none">
                      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
                    </svg>
                    发送中
                  </span>
                  <span v-else-if="sendCooldown > 0" class="font-mono text-[11px] text-zinc-500">
                    {{ sendCooldown }}s 后重试
                  </span>
                  <span v-else class="text-[11px]">
                    获取验证码
                  </span>
                </button>
              </div>
            </div>
          </div>

          <!-- 设置密码 (独立一行) -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <label class="text-xs font-medium text-zinc-700">设置密码</label>
            </div>
            <div class="relative">
              <input
                v-model="regPassword"
                :type="showRegPassword ? 'text' : 'password'"
                required
                minlength="6"
                autocomplete="new-password"
                placeholder="设置登录密码 (至少 6 位)"
                :class="[
                  'w-full pl-3 pr-10 py-2 text-xs bg-white border rounded-lg focus:outline-none focus:ring-2 transition-all font-mono',
                  regPassword && regPassword.length < 6
                    ? 'border-amber-300 focus:ring-amber-500'
                    : 'border-zinc-200 focus:ring-zinc-900 focus:border-transparent'
                ]"
              />
              <button
                type="button"
                @click="showRegPassword = !showRegPassword"
                tabindex="-1"
                class="absolute right-2.5 top-1/2 -translate-y-1/2 flex items-center justify-center text-zinc-400 hover:text-zinc-700 transition-colors p-1 cursor-pointer focus:outline-none"
                :title="showRegPassword ? '隐藏密码明文' : '显示密码明文'"
              >
                <svg v-if="!showRegPassword" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z"></path>
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                </svg>
                <svg v-else class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M3.98 8.223A10.477 10.477 0 001.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.451 10.451 0 0112 4.5c4.756 0 8.773 3.162 10.065 7.498a10.522 10.522 0 01-4.293 5.774M6.228 6.228L3 3m3.228 3.228l3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 10-4.243-4.243m4.242 4.242L9.88 9.88"></path>
                </svg>
              </button>
            </div>
            <p v-if="regPassword && regPassword.length < 6" class="text-[10px] text-amber-600 mt-1">
              密码需至少 6 位 (当前 {{ regPassword.length }} 位)
            </p>
          </div>

          <!-- 确认密码 (独立一行) -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <label class="text-xs font-medium text-zinc-700">确认密码</label>
            </div>
            <div class="relative">
              <input
                v-model="regConfirmPassword"
                :type="showRegConfirmPassword ? 'text' : 'password'"
                required
                minlength="6"
                autocomplete="new-password"
                placeholder="再次输入以确认登录密码"
                :class="[
                  'w-full pl-3 pr-10 py-2 text-xs bg-white border rounded-lg focus:outline-none focus:ring-2 transition-all font-mono',
                  regConfirmPassword && regConfirmPassword !== regPassword
                    ? 'border-rose-300 focus:ring-rose-500'
                    : 'border-zinc-200 focus:ring-zinc-900 focus:border-transparent'
                ]"
              />
              <button
                type="button"
                @click="showRegConfirmPassword = !showRegConfirmPassword"
                tabindex="-1"
                class="absolute right-2.5 top-1/2 -translate-y-1/2 flex items-center justify-center text-zinc-400 hover:text-zinc-700 transition-colors p-1 cursor-pointer focus:outline-none"
                :title="showRegConfirmPassword ? '隐藏密码明文' : '显示密码明文'"
              >
                <svg v-if="!showRegConfirmPassword" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z"></path>
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                </svg>
                <svg v-else class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M3.98 8.223A10.477 10.477 0 001.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.451 10.451 0 0112 4.5c4.756 0 8.773 3.162 10.065 7.498a10.522 10.522 0 01-4.293 5.774M6.228 6.228L3 3m3.228 3.228l3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 10-4.243-4.243m4.242 4.242L9.88 9.88"></path>
                </svg>
              </button>
            </div>
            <p
              v-if="regConfirmPassword && regConfirmPassword !== regPassword"
              class="text-[10px] text-rose-500 mt-1"
            >
              两次输入的密码不一致
            </p>
            <p
              v-else-if="regConfirmPassword && regConfirmPassword === regPassword && regPassword.length >= 6"
              class="text-[10px] text-emerald-600 mt-1"
            >
              密码一致 ✓
            </p>
          </div>

          <button
            type="submit"
            :disabled="!isRegisterValid"
            :class="[
              'w-full py-2.5 px-4 rounded-lg text-xs font-medium transition-all shadow-xs mt-3 flex items-center justify-center gap-1.5',
              isRegisterValid
                ? 'bg-zinc-900 text-white hover:bg-zinc-800 cursor-pointer active:scale-[0.99]'
                : 'bg-zinc-100 text-zinc-400 cursor-not-allowed border border-zinc-200/80 shadow-none'
            ]"
          >
            <svg v-if="loading" class="animate-spin h-3.5 w-3.5 text-zinc-400" viewBox="0 0 24 24" fill="none">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
            </svg>
            <span>{{ loading ? '处理注册中...' : '立即注册' }}</span>
          </button>
        </form>
      </div>
    </div>

    <!-- 注册成功: 用户初始化配置与偏好领域挑选弹窗 (Onboarding Modal) -->
    <div v-if="showOnboardingModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs px-4 animate-in fade-in duration-200">
      <div class="bg-white rounded-xl shadow-2xl border border-zinc-200 w-full max-w-md p-6 relative">
        <div class="flex items-center gap-3 mb-4">
          <div class="w-10 h-10 rounded-full bg-emerald-50 text-emerald-600 flex items-center justify-center text-sm font-bold border border-emerald-200/60 shadow-2xs">
            ✓
          </div>
          <div>
            <h3 class="text-sm font-semibold text-zinc-900">注册成功 · 欢迎加入 AttackBox</h3>
            <p class="text-xs text-zinc-500 mt-0.5">默认系统编号：<span class="font-mono font-medium text-zinc-800">{{ registeredUser?.username }}</span></p>
          </div>
        </div>

        <form @submit.prevent="handleCompleteOnboarding" class="space-y-4 pt-1">
          <!-- 1. 自定义昵称修改 -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <label class="text-xs font-medium text-zinc-700">自定义平台昵称</label>
              <span class="text-[10px] text-zinc-400">2-24 位字符 (选填，后续可在个人中心修改)</span>
            </div>
            <input
              v-model.trim="customNickname"
              type="text"
              maxlength="24"
              placeholder="输入个性化用户名或保留默认"
              class="w-full px-3 py-2 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 focus:border-transparent font-mono transition-all"
            />
          </div>

          <!-- 2. 攻防偏好领域挑选 -->
          <div>
            <label class="block text-xs font-medium text-zinc-700 mb-2">挑选您感兴趣的安全技术领域 (多选)</label>
            <div class="grid grid-cols-2 gap-2">
              <button
                v-for="domain in domainOptions"
                :key="domain.id"
                type="button"
                @click="toggleDomain(domain.id)"
                :class="[
                  'px-3 py-2 rounded-lg text-xs font-medium border text-left transition-all flex items-center justify-between cursor-pointer active:scale-[0.98]',
                  selectedDomains.includes(domain.id)
                    ? 'bg-zinc-900 text-white border-zinc-900 shadow-2xs'
                    : 'bg-zinc-50 text-zinc-700 border-zinc-200 hover:bg-zinc-100'
                ]"
              >
                <span>{{ domain.label }}</span>
                <span v-if="selectedDomains.includes(domain.id)" class="text-[10px] text-zinc-300">✓</span>
              </button>
            </div>
          </div>

          <div class="flex items-center gap-2 pt-3 border-t border-zinc-100">
            <button
              type="button"
              @click="skipOnboarding"
              class="flex-1 py-2 px-3 text-xs font-medium rounded-lg border border-zinc-200 text-zinc-600 hover:bg-zinc-50 transition-all cursor-pointer"
            >
              使用默认设置
            </button>
            <button
              type="submit"
              :disabled="onboardingLoading"
              class="flex-1 py-2 px-3 text-xs font-medium rounded-lg bg-zinc-900 text-white hover:bg-zinc-800 disabled:opacity-50 transition-all cursor-pointer shadow-xs"
            >
              <span v-if="onboardingLoading">保存中...</span>
              <span v-else>完成配置并进入平台</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { api } from '../api.js'

const emit = defineEmits(['auth-success', 'show-toast'])

const authMode = ref('login')
const allowRegister = ref(true)
const loading = ref(false)

// 登录表单
const loginEmail = ref('')
const loginPassword = ref('')
const showLoginPassword = ref(false)

// 注册表单
const regEmail = ref('')
const regPassword = ref('')
const regConfirmPassword = ref('')
const verificationCode = ref('')
const showRegPassword = ref(false)
const showRegConfirmPassword = ref(false)

// 验证码倒计时与提示状态
const sendingCode = ref(false)
const sendCooldown = ref(0)
const codeSentNotice = ref(false)
let cooldownTimer = null

// 注册后配置引导弹窗
const showOnboardingModal = ref(false)
const registeredUser = ref(null)
const customNickname = ref('')
const onboardingLoading = ref(false)

// 表单有效性前置计算 (严格控制按钮状态与光标样式)
const canSendCode = computed(() => {
  return (
    isValidEmail(regEmail.value) &&
    sendCooldown.value <= 0 &&
    !sendingCode.value
  )
})

const isLoginValid = computed(() => {
  return (
    isValidEmail(loginEmail.value) &&
    loginPassword.value.length >= 6 &&
    !loading.value
  )
})

const isRegisterValid = computed(() => {
  return (
    isValidEmail(regEmail.value) &&
    verificationCode.value.length === 6 &&
    regPassword.value.length >= 6 &&
    regConfirmPassword.value.length >= 6 &&
    regConfirmPassword.value === regPassword.value &&
    !loading.value
  )
})

// 领域偏好选项
const domainOptions = [
  { id: 'web', label: 'Web 安全 / 渗透测试' },
  { id: 'pwn', label: '二进制逆向 / PWN' },
  { id: 'ad', label: '内网渗透 / AD 域控' },
  { id: 'cloud', label: '云安全 / 容器逃逸' },
  { id: 'crypto', label: '密码学 / 杂项 Misc' },
  { id: 'redteam', label: '红蓝对抗 / 漏洞挖掘' },
]
const selectedDomains = ref(['web'])

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

function switchAuthMode(mode) {
  authMode.value = mode
  codeSentNotice.value = false
}

function handleCodeInput(e) {
  // 严格过滤非数字字符
  const cleaned = e.target.value.replace(/\D/g, '')
  verificationCode.value = cleaned
}

function toggleDomain(domainId) {
  const idx = selectedDomains.value.indexOf(domainId)
  if (idx >= 0) {
    if (selectedDomains.value.length > 1) {
      selectedDomains.value.splice(idx, 1)
    }
  } else {
    selectedDomains.value.push(domainId)
  }
}

async function handleSendCode() {
  if (!regEmail.value) {
    emit('show-toast', '请先输入注册邮箱')
    return
  }
  if (!isValidEmail(regEmail.value)) {
    emit('show-toast', '仅支持 QQ 邮箱、谷歌 Gmail 及微软 Outlook 邮箱')
    return
  }

  sendingCode.value = true
  try {
    const data = await api.sendVerificationCode(regEmail.value)
    emit('show-toast', data.message || '验证码已发送至收件箱')
    codeSentNotice.value = true
    sendCooldown.value = 60
    if (cooldownTimer) clearInterval(cooldownTimer)
    cooldownTimer = setInterval(() => {
      sendCooldown.value--
      if (sendCooldown.value <= 0) {
        clearInterval(cooldownTimer)
      }
    }, 1000)
  } catch (err) {
    emit('show-toast', err.message || '验证码发送失败')
  } finally {
    sendingCode.value = false
  }
}

async function handleLogin() {
  if (!loginEmail.value || !loginPassword.value) {
    emit('show-toast', '请输入安全邮箱与密码')
    return
  }
  if (!isValidEmail(loginEmail.value)) {
    emit('show-toast', '仅支持 QQ 邮箱、谷歌 Gmail 及微软 Outlook 邮箱')
    return
  }

  loading.value = true
  try {
    const data = await api.login(loginEmail.value, loginPassword.value)
    if (data.token) {
      api.setToken(data.token)
    }
    emit('show-toast', '登录成功')
    emit('auth-success', data.user)
  } catch (err) {
    emit('show-toast', err.message || '登录失败，请检查邮箱与密码')
  } finally {
    loading.value = false
  }
}

async function handleRegister() {
  if (!regEmail.value) {
    emit('show-toast', '请输入注册安全邮箱')
    return
  }
  if (!isValidEmail(regEmail.value)) {
    emit('show-toast', '仅支持 QQ 邮箱、谷歌 Gmail 及微软 Outlook 邮箱')
    return
  }
  if (!verificationCode.value || verificationCode.value.length !== 6) {
    emit('show-toast', '请输入 6 位邮箱验证码')
    return
  }
  if (!regPassword.value || regPassword.value.length < 6) {
    emit('show-toast', '密码长度至少需 6 位')
    return
  }
  if (regPassword.value !== regConfirmPassword.value) {
    emit('show-toast', '两次输入的密码不一致')
    return
  }

  loading.value = true
  try {
    const data = await api.register(regEmail.value, verificationCode.value, regPassword.value)
    if (data.token) {
      api.setToken(data.token)
    }
    registeredUser.value = data.user
    customNickname.value = data.user.username || ''
    showOnboardingModal.value = true
    emit('show-toast', '账号注册成功，请完善初始化配置')
  } catch (err) {
    emit('show-toast', err.message || '注册失败')
  } finally {
    loading.value = false
  }
}

async function handleCompleteOnboarding() {
  onboardingLoading.value = true
  try {
    const newNick = customNickname.value.trim()
    if (newNick && newNick !== registeredUser.value?.username) {
      await api.updateUsername(newNick)
      if (registeredUser.value) {
        registeredUser.value.username = newNick
      }
    }
    localStorage.setItem('attackbox_domains', JSON.stringify(selectedDomains.value))
    showOnboardingModal.value = false
    emit('show-toast', '配置完成，欢迎进入 AttackBox！')
    emit('auth-success', registeredUser.value)
  } catch (err) {
    emit('show-toast', err.message || '更新用户名失败')
  } finally {
    onboardingLoading.value = false
  }
}

function skipOnboarding() {
  localStorage.setItem('attackbox_domains', JSON.stringify(selectedDomains.value))
  showOnboardingModal.value = false
  emit('show-toast', '已应用默认设置')
  emit('auth-success', registeredUser.value)
}

onMounted(async () => {
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
})

onBeforeUnmount(() => {
  if (cooldownTimer) clearInterval(cooldownTimer)
})
</script>
