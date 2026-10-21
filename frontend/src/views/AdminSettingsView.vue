<template>
  <section class="space-y-6 max-w-3xl">
    <div>
      <h2 class="text-base font-semibold text-zinc-900">系统设置与全局策略</h2>
      <p class="text-xs text-zinc-500 mt-0.5">统一配置底层沙箱生命周期、访问控制及全站公告广播</p>
    </div>

    <!-- 1. 全局系统策略设置卡片 -->
    <div class="shadcn-card p-5 space-y-4">
      <div class="pb-3 border-b border-zinc-100 flex items-center justify-between">
        <div>
          <h3 class="text-xs font-mono font-semibold text-zinc-900 tracking-wider">SYSTEM POLICIES</h3>
          <p class="text-[11px] text-zinc-500 mt-0.5">沙箱配额、生命周期与账号自主注册准入控制</p>
        </div>
      </div>

      <form @submit.prevent="saveSettings" class="space-y-4 text-xs">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label class="block font-medium text-zinc-700 mb-1 font-mono">全局最大沙箱槽位 (Max Slots)</label>
            <input
              v-model.number="policyForm.maxSlots"
              type="number"
              min="1"
              max="100"
              required
              class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono text-xs"
            />
            <p class="text-[11px] text-zinc-400 mt-1">控制 VPS 能够并发运行的最大隔离沙箱数量</p>
          </div>

          <div>
            <label class="block font-medium text-zinc-700 mb-1 font-mono">单沙箱默认租期 (TTL, 分钟)</label>
            <input
              v-model.number="policyForm.ttlMinutes"
              type="number"
              min="1"
              max="1440"
              required
              class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono text-xs"
            />
            <p class="text-[11px] text-zinc-400 mt-1">沙箱启动起算的最长可用时长，超时强制回收</p>
          </div>
        </div>

        <div>
          <label class="block font-medium text-zinc-700 mb-1 font-mono">无交互空闲自动释放超时 (分钟)</label>
          <input
            v-model.number="policyForm.idleTimeoutMinutes"
            type="number"
            min="1"
            max="120"
            required
            class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono text-xs"
          />
          <p class="text-[11px] text-zinc-400 mt-1">终端/GUI 无任何键盘鼠标操作达此时长自动回收 VPS 内存</p>
        </div>

        <div class="pt-3 border-t border-zinc-100 flex items-center justify-between">
          <div>
            <div class="font-medium text-zinc-800">开放新用户自主注册</div>
            <div class="text-[11px] text-zinc-400">关闭后登录页面的注册入口与接口将被禁止</div>
          </div>
          <button
            type="button"
            @click="policyForm.allowRegister = !policyForm.allowRegister"
            :class="[
              'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out',
              policyForm.allowRegister ? 'bg-zinc-900' : 'bg-zinc-200'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                policyForm.allowRegister ? 'translate-x-4' : 'translate-x-0'
              ]"
            />
          </button>
        </div>

        <div class="pt-2 flex justify-end">
          <button
            type="submit"
            :disabled="savingPolicy"
            class="px-4 py-2 rounded-lg text-xs font-semibold shadcn-btn-primary disabled:opacity-50 cursor-pointer"
          >
            {{ savingPolicy ? '保存中...' : '保存策略' }}
          </button>
        </div>
      </form>
    </div>

    <!-- 2. 全站公告通知设置卡片 -->
    <div class="shadcn-card p-5 space-y-4">
      <div class="pb-3 border-b border-zinc-100 flex items-center justify-between">
        <div>
          <h3 class="text-xs font-mono font-semibold text-zinc-900 tracking-wider">GLOBAL BROADCAST</h3>
          <p class="text-[11px] text-zinc-500 mt-0.5">向所有登录用户顶部推送实时公告横幅</p>
        </div>
      </div>

      <form @submit.prevent="saveBroadcast" class="space-y-4 text-xs">
        <div class="flex items-center justify-between">
          <div>
            <div class="font-medium text-zinc-800">开启顶部横幅通知</div>
            <div class="text-[11px] text-zinc-400">关闭后用户端将不展示全站通知横幅</div>
          </div>
          <button
            type="button"
            @click="broadcastForm.enabled = !broadcastForm.enabled"
            :class="[
              'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out',
              broadcastForm.enabled ? 'bg-zinc-900' : 'bg-zinc-200'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                broadcastForm.enabled ? 'translate-x-4' : 'translate-x-0'
              ]"
            />
          </button>
        </div>

        <div>
          <label class="block font-medium text-zinc-700 mb-1.5 font-mono">通知内容</label>
          <textarea
            v-model.trim="broadcastForm.message"
            rows="2"
            required
            placeholder="输入推送给所有用户的通知消息..."
            class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 text-xs"
          ></textarea>
        </div>

        <div class="pt-2 flex justify-end">
          <button
            type="submit"
            :disabled="savingBroadcast"
            class="px-4 py-2 rounded-lg text-xs font-semibold shadcn-btn-primary disabled:opacity-50 cursor-pointer"
          >
            {{ savingBroadcast ? '发布中...' : '保存并发布' }}
          </button>
        </div>
      </form>
    </div>

    <!-- 3. OAuth 第三方登录配置 (AES-256-GCM 密文存储) -->
    <div class="shadcn-card p-5 space-y-6">
      <div class="pb-3 border-b border-zinc-100 flex items-center justify-between">
        <div>
          <h3 class="text-xs font-mono font-semibold text-zinc-900 tracking-wider">OAUTH PROVIDERS (AES-256-GCM)</h3>
          <p class="text-[11px] text-zinc-500 mt-0.5">配置 GitHub 与 Outlook 凭据，敏感 Secret 经密码学 AES-GCM 密文存储，永不回显明文</p>
        </div>
      </div>

      <!-- GitHub 配置 -->
      <form @submit.prevent="saveOAuth('github')" class="space-y-3 p-4 rounded-xl border border-zinc-200/80 bg-zinc-50/50 text-xs">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <svg class="w-4 h-4 fill-zinc-900" viewBox="0 0 24 24">
              <path fill-rule="evenodd" clip-rule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"></path>
            </svg>
            <span class="font-semibold text-zinc-900 font-mono">GitHub OAuth App</span>
            <span v-if="oauthConfigs.github?.configured" class="px-1.5 py-0.5 rounded text-[10px] bg-emerald-100 text-emerald-800 font-medium">已配置</span>
            <span v-else class="px-1.5 py-0.5 rounded text-[10px] bg-zinc-200 text-zinc-600 font-medium">未配置</span>
          </div>

          <label class="flex items-center gap-1.5 cursor-pointer">
            <span class="text-[11px] text-zinc-600">启用</span>
            <input type="checkbox" v-model="oauthConfigs.github.enabled" class="rounded border-zinc-300 text-zinc-900 focus:ring-zinc-900" />
          </label>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
          <div>
            <label class="block font-medium text-zinc-700 mb-1 font-mono">Client ID</label>
            <input
              v-model.trim="oauthConfigs.github.client_id"
              type="text"
              placeholder="Iv1.xxxxxxxxxxxx"
              class="w-full px-3 py-1.5 border border-zinc-200 rounded-lg bg-white focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono text-xs"
            />
          </div>
          <div>
            <label class="block font-medium text-zinc-700 mb-1 font-mono">Client Secret</label>
            <input
              v-model.trim="oauthSecrets.github"
              type="password"
              :placeholder="oauthConfigs.github?.configured ? '•••••••••••••••• (已加密，留空保持现有)' : '输入 GitHub 平台生成的 Secret'"
              class="w-full px-3 py-1.5 border border-zinc-200 rounded-lg bg-white focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono text-xs"
            />
          </div>
        </div>

        <div class="flex items-center justify-between pt-1">
          <p class="text-[10px] text-zinc-400 font-mono truncate">
            回调地址: <span class="text-zinc-600 select-all">{{ callbackBaseUrl }}/api/oauth/github/callback</span>
          </p>
          <button
            type="submit"
            :disabled="savingOAuth.github"
            class="px-3 py-1.5 rounded-lg text-xs font-semibold shadcn-btn-primary disabled:opacity-50 cursor-pointer"
          >
            {{ savingOAuth.github ? '保存中...' : '保存 GitHub' }}
          </button>
        </div>
      </form>

      <!-- Outlook (Microsoft) 配置 -->
      <form @submit.prevent="saveOAuth('outlook')" class="space-y-3 p-4 rounded-xl border border-zinc-200/80 bg-zinc-50/50 text-xs">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <svg class="w-4 h-4" viewBox="0 0 21 21">
              <rect x="1" y="1" width="9" height="9" fill="#f25022"/>
              <rect x="11" y="1" width="9" height="9" fill="#7fba00"/>
              <rect x="1" y="11" width="9" height="9" fill="#00a4ef"/>
              <rect x="11" y="11" width="9" height="9" fill="#ffb900"/>
            </svg>
            <span class="font-semibold text-zinc-900 font-mono">Outlook / Microsoft Entra ID</span>
            <span v-if="oauthConfigs.outlook?.configured" class="px-1.5 py-0.5 rounded text-[10px] bg-emerald-100 text-emerald-800 font-medium">已配置</span>
            <span v-else class="px-1.5 py-0.5 rounded text-[10px] bg-zinc-200 text-zinc-600 font-medium">未配置</span>
          </div>

          <label class="flex items-center gap-1.5 cursor-pointer">
            <span class="text-[11px] text-zinc-600">启用</span>
            <input type="checkbox" v-model="oauthConfigs.outlook.enabled" class="rounded border-zinc-300 text-zinc-900 focus:ring-zinc-900" />
          </label>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
          <div>
            <label class="block font-medium text-zinc-700 mb-1 font-mono">Application (client) ID</label>
            <input
              v-model.trim="oauthConfigs.outlook.client_id"
              type="text"
              placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
              class="w-full px-3 py-1.5 border border-zinc-200 rounded-lg bg-white focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono text-xs"
            />
          </div>
          <div>
            <label class="block font-medium text-zinc-700 mb-1 font-mono">Client Secret Value</label>
            <input
              v-model.trim="oauthSecrets.outlook"
              type="password"
              :placeholder="oauthConfigs.outlook?.configured ? '•••••••••••••••• (已加密，留空保持现有)' : '输入 Azure 应用机密值'"
              class="w-full px-3 py-1.5 border border-zinc-200 rounded-lg bg-white focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono text-xs"
            />
          </div>
        </div>

        <div class="flex items-center justify-between pt-1">
          <p class="text-[10px] text-zinc-400 font-mono truncate">
            重定向 URI: <span class="text-zinc-600 select-all">{{ callbackBaseUrl }}/api/oauth/outlook/callback</span>
          </p>
          <button
            type="submit"
            :disabled="savingOAuth.outlook"
            class="px-3 py-1.5 rounded-lg text-xs font-semibold shadcn-btn-primary disabled:opacity-50 cursor-pointer"
          >
            {{ savingOAuth.outlook ? '保存中...' : '保存 Outlook' }}
          </button>
        </div>
      </form>
    </div>

    <!-- 4. Resend 邮件分发服务 (信封加密 · 零落地存储) -->
    <div class="shadcn-card p-5 space-y-4">
      <div class="pb-3 border-b border-zinc-100 flex items-center justify-between">
        <div>
          <h3 class="text-xs font-mono font-semibold text-zinc-900 tracking-wider">RESEND EMAIL SERVICE</h3>
          <p class="text-[11px] text-zinc-500 mt-0.5">用于注册验证码发送。API Key 经管理员密码派生密钥加密落库，内存安全解封，零明文落地</p>
        </div>
        <div>
          <span v-if="resendForm.configured" class="px-1.5 py-0.5 rounded text-[10px] bg-emerald-100 text-emerald-800 font-medium">已配置</span>
          <span v-else class="px-1.5 py-0.5 rounded text-[10px] bg-zinc-200 text-zinc-600 font-medium">未配置</span>
        </div>
      </div>

      <form @submit.prevent="saveResend" class="space-y-4 text-xs">
        <div class="flex items-center justify-between">
          <div>
            <div class="font-medium text-zinc-800">启用 Resend 邮件分发服务</div>
            <div class="text-[11px] text-zinc-400">开启后新用户注册将强制要求邮箱验证码核验</div>
          </div>
          <button
            type="button"
            @click="resendForm.enabled = !resendForm.enabled"
            :class="[
              'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out',
              resendForm.enabled ? 'bg-zinc-900' : 'bg-zinc-200'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                resendForm.enabled ? 'translate-x-4' : 'translate-x-0'
              ]"
            />
          </button>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label class="block font-medium text-zinc-700 mb-1 font-mono">发件人地址 (From Header)</label>
            <input
              v-model.trim="resendForm.from"
              type="text"
              required
              placeholder="AttackBox <onboarding@resend.dev>"
              class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono text-xs"
            />
            <p class="text-[11px] text-zinc-400 mt-1">例如: AttackBox &lt;auth@yourdomain.com&gt; 或 Resend 默认测试域</p>
          </div>

          <div>
            <label class="block font-medium text-zinc-700 mb-1 font-mono">Resend API Key</label>
            <input
              v-model.trim="resendApiKey"
              type="password"
              :placeholder="resendForm.configured ? '•••••••••••••••• (已信封加密，留空保持现有)' : 're_xxxxxxxxxxxxxxxx'"
              class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono text-xs"
            />
            <p class="text-[11px] text-zinc-400 mt-1">API Key 仅在内存持有，接口零明文回显</p>
          </div>
        </div>

        <div class="pt-2 flex justify-end">
          <button
            type="submit"
            :disabled="savingResend"
            class="px-4 py-2 rounded-lg text-xs font-semibold shadcn-btn-primary disabled:opacity-50 cursor-pointer"
          >
            {{ savingResend ? '保存中...' : '保存 Resend 配置' }}
          </button>
        </div>
      </form>
    </div>

    <!-- 5. Cloudflare Turnstile 人机防护配置 (AES-256-GCM 密文存储) -->
    <div class="shadcn-card p-5 space-y-4">
      <div class="pb-3 border-b border-zinc-100 flex items-center justify-between">
        <div>
          <h3 class="text-xs font-mono font-semibold text-zinc-900 tracking-wider">CLOUDFLARE TURNSTILE CAPTCHA</h3>
          <p class="text-[11px] text-zinc-500 mt-0.5">防御暴力破解与邮件验证码接口滥用消耗，密钥以 AES-256-GCM 信封加密</p>
        </div>
        <span
          v-if="turnstileForm.configured"
          class="text-[10px] font-mono px-2 py-0.5 bg-emerald-50 text-emerald-700 border border-emerald-200 rounded-full"
        >
          已配置密匙
        </span>
        <span
          v-else
          class="text-[10px] font-mono px-2 py-0.5 bg-amber-50 text-amber-700 border border-amber-200 rounded-full"
        >
          未配置密匙
        </span>
      </div>

      <form @submit.prevent="saveTurnstile" class="space-y-4 text-xs">
        <div class="flex items-center justify-between">
          <div>
            <div class="font-medium text-zinc-800">启用 Cloudflare Turnstile 人机验证</div>
            <div class="text-[11px] text-zinc-400">开启后登录表单与注册发信接口必须通过 Turnstile 人机核验</div>
          </div>
          <button
            type="button"
            @click="turnstileForm.enabled = !turnstileForm.enabled"
            :class="[
              'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out',
              turnstileForm.enabled ? 'bg-zinc-900' : 'bg-zinc-200'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                turnstileForm.enabled ? 'translate-x-4' : 'translate-x-0'
              ]"
            />
          </button>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label class="block font-medium text-zinc-700 mb-1 font-mono">Site Key (前端公钥)</label>
            <input
              v-model.trim="turnstileForm.site_key"
              type="text"
              class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono text-xs"
            />
            <p class="text-[11px] text-zinc-400 mt-1">Cloudflare Turnstile 站点公钥 (例如: 0x4AAAAAA...)</p>
          </div>

          <div>
            <label class="block font-medium text-zinc-700 mb-1 font-mono">Secret Key (后端私钥)</label>
            <input
              v-model.trim="turnstileSecretKey"
              type="password"
              class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 font-mono text-xs"
            />
            <p class="text-[11px] text-zinc-400 mt-1">密钥仅在后端内存持有并信封加密落库，前端零明文回显</p>
          </div>
        </div>

        <div class="pt-2 flex justify-end">
          <button
            type="submit"
            :disabled="savingTurnstile"
            class="px-4 py-2 rounded-lg text-xs font-semibold shadcn-btn-primary disabled:opacity-50 cursor-pointer"
          >
            {{ savingTurnstile ? '保存中...' : '保存 Turnstile 配置' }}
          </button>
        </div>
      </form>
    </div>
  </section>

</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import { adminApi as api } from '../adminApi.js'

const props = defineProps({
  broadcast: {
    type: Object,
    default: () => ({ enabled: false, message: '' }),
  },
})

const emit = defineEmits(['show-toast', 'broadcast-updated'])

const policyForm = ref({
  maxSlots: 20,
  ttlMinutes: 120,
  idleTimeoutMinutes: 10,
  allowRegister: true,
})
const savingPolicy = ref(false)

const broadcastForm = ref({
  enabled: props.broadcast.enabled || false,
  message: props.broadcast.message || '',
})
const savingBroadcast = ref(false)

watch(
  () => props.broadcast,
  (newVal) => {
    if (newVal) {
      broadcastForm.value.enabled = newVal.enabled || false
      broadcastForm.value.message = newVal.message || ''
    }
  },
  { deep: true }
)

async function loadSettings() {
  try {
    const data = await api.getSettings()
    policyForm.value = {
      maxSlots: data.maxSlots || 20,
      ttlMinutes: data.ttlMinutes || 120,
      idleTimeoutMinutes: data.idleTimeoutMinutes || 10,
      allowRegister: data.allowRegister !== false,
    }
  } catch (e) {
    emit('show-toast', '读取全局系统策略失败')
  }
}

async function saveSettings() {
  savingPolicy.value = true
  try {
    const res = await api.updateSettings(policyForm.value)
    if (!res.success) {
      emit('show-toast', res.error || '保存策略失败')
      return
    }
    emit('show-toast', '系统策略配置已更新生效')
  } catch (e) {
    emit('show-toast', '保存策略异常: ' + e.message)
  } finally {
    savingPolicy.value = false
  }
}

async function saveBroadcast() {
  savingBroadcast.value = true
  try {
    await api.saveBroadcast(broadcastForm.value.enabled, broadcastForm.value.message)
    emit('broadcast-updated', {
      enabled: broadcastForm.value.enabled,
      message: broadcastForm.value.message,
    })
    emit('show-toast', '全局广播通知已更新并发布')
  } catch (e) {
    emit('show-toast', '发布广播失败: ' + e.message)
  } finally {
    savingBroadcast.value = false
  }
}

// OAuth 状态与方法
const callbackBaseUrl = ref(window.location.origin)
const oauthConfigs = ref({
  github: { provider: 'github', client_id: '', enabled: false, configured: false },
  outlook: { provider: 'outlook', client_id: '', enabled: false, configured: false },
})
const oauthSecrets = ref({
  github: '',
  outlook: '',
})
const savingOAuth = ref({
  github: false,
  outlook: false,
})

async function loadOAuth() {
  try {
    const res = await api.getOAuthSettings()
    if (res.success && res.configs) {
      if (res.configs.github) oauthConfigs.value.github = { ...res.configs.github }
      if (res.configs.outlook) oauthConfigs.value.outlook = { ...res.configs.outlook }
    }
  } catch (e) {
    emit('show-toast', '读取 OAuth 配置失败: ' + e.message)
  }
}

async function saveOAuth(provider) {
  savingOAuth.value[provider] = true
  try {
    const cfg = oauthConfigs.value[provider]
    const secret = oauthSecrets.value[provider]
    const res = await api.updateOAuthSettings({
      provider,
      client_id: cfg.client_id,
      client_secret: secret,
      enabled: cfg.enabled,
    })
    if (!res.success) {
      emit('show-toast', res.error || '保存 OAuth 配置失败')
      return
    }
    emit('show-toast', `${provider === 'github' ? 'GitHub' : 'Outlook'} 配置已安全加密保存`)
    oauthSecrets.value[provider] = ''
    await loadOAuth()
  } catch (e) {
    emit('show-toast', '保存异常: ' + e.message)
  } finally {
    savingOAuth.value[provider] = false
  }
}

// Resend 邮件服务状态与方法
const resendForm = ref({
  enabled: false,
  from: 'AttackBox <onboarding@resend.dev>',
  configured: false,
})
const resendApiKey = ref('')
const savingResend = ref(false)

async function loadResend() {
  try {
    const res = await api.getResendSettings()
    if (res.success && res.config) {
      resendForm.value = {
        enabled: !!res.config.enabled,
        from: res.config.from || 'AttackBox <onboarding@resend.dev>',
        configured: !!res.config.configured,
      }
    }
  } catch (e) {
    emit('show-toast', '读取 Resend 邮件配置失败: ' + e.message)
  }
}

async function saveResend() {
  savingResend.value = true
  try {
    const res = await api.updateResendSettings({
      enabled: resendForm.value.enabled,
      from: resendForm.value.from,
      api_key: resendApiKey.value,
    })
    if (!res.success) {
      emit('show-toast', res.error || '保存 Resend 配置失败')
      return
    }
    emit('show-toast', 'Resend 邮件服务配置已信封加密落库')
    resendApiKey.value = ''
    await loadResend()
  } catch (e) {
    emit('show-toast', '保存 Resend 配置异常: ' + e.message)
  } finally {
    savingResend.value = false
  }
}

// Turnstile 状态与方法
const turnstileForm = ref({
  enabled: false,
  site_key: '',
  configured: false,
})
const turnstileSecretKey = ref('')
const savingTurnstile = ref(false)

async function loadTurnstile() {
  try {
    const res = await api.getTurnstileSettings()
    if (res.success && res.config) {
      turnstileForm.value = {
        enabled: !!res.config.enabled,
        site_key: res.config.site_key || '',
        configured: !!res.config.configured,
      }
    }
  } catch (e) {
    emit('show-toast', '读取 Turnstile 配置失败: ' + e.message)
  }
}

async function saveTurnstile() {
  savingTurnstile.value = true
  try {
    const res = await api.updateTurnstileSettings({
      enabled: turnstileForm.value.enabled,
      site_key: turnstileForm.value.site_key,
      secret_key: turnstileSecretKey.value,
    })
    if (!res.success) {
      emit('show-toast', res.error || '保存 Turnstile 配置失败')
      return
    }
    emit('show-toast', 'Turnstile 人机防护配置已信封加密落库')
    turnstileSecretKey.value = ''
    await loadTurnstile()
  } catch (e) {
    emit('show-toast', '保存 Turnstile 配置异常: ' + e.message)
  } finally {
    savingTurnstile.value = false
  }
}

onMounted(() => {
  loadSettings()
  loadOAuth()
  loadResend()
  loadTurnstile()
})
</script>

