<template>
  <div class="min-h-full flex flex-col flex-1">
    <!-- 顶部导航 -->
    <header class="bg-white border-b border-zinc-200 sticky top-0 z-30">
      <div class="max-w-5xl mx-auto px-4 sm:px-6 h-14 flex items-center justify-between gap-4">
        <div class="flex items-center gap-6">
          <div class="flex items-center gap-2">
            <img src="/favicon.ico" alt="AttackBox Logo" class="w-8 h-8 rounded-md object-contain" />
            <span class="font-semibold text-sm tracking-tight text-zinc-900 font-mono">AttackBox ADMIN</span>
          </div>

          <nav class="hidden sm:flex items-center gap-1">
            <button
              @click="switchTab('sandboxes')"
              :class="[
                'px-2.5 py-1.5 rounded-md text-xs font-mono font-medium transition-all cursor-pointer',
                activeTab === 'sandboxes'
                  ? 'bg-zinc-900 text-white shadow-sm'
                  : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900'
              ]"
            >
              SANDBOXES
            </button>
            <button
              @click="switchTab('users')"
              :class="[
                'px-2.5 py-1.5 rounded-md text-xs font-mono font-medium transition-all cursor-pointer',
                activeTab === 'users'
                  ? 'bg-zinc-900 text-white shadow-sm'
                  : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900'
              ]"
            >
              USERS
            </button>
            <button
              @click="switchTab('audit')"
              :class="[
                'px-2.5 py-1.5 rounded-md text-xs font-mono font-medium transition-all cursor-pointer',
                activeTab === 'audit'
                  ? 'bg-zinc-900 text-white shadow-sm'
                  : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900'
              ]"
            >
              AUDIT
            </button>
            <button
              @click="switchTab('nodes')"
              :class="[
                'px-2.5 py-1.5 rounded-md text-xs font-mono font-medium transition-all cursor-pointer',
                activeTab === 'nodes'
                  ? 'bg-zinc-900 text-white shadow-sm'
                  : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900'
              ]"
            >
              METRICS
            </button>
            <button
              @click="switchTab('settings')"
              :class="[
                'px-2.5 py-1.5 rounded-md text-xs font-mono font-medium transition-all cursor-pointer',
                activeTab === 'settings'
                  ? 'bg-zinc-900 text-white shadow-sm'
                  : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900'
              ]"
            >
              SETTINGS
            </button>
            <a
              href="/"
              class="px-2.5 py-1.5 rounded-md text-xs font-mono font-medium text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 transition-all cursor-pointer flex items-center gap-1"
            >
              <span>WORKSPACE</span>
              <svg class="w-3 h-3 text-zinc-400" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"></path>
              </svg>
            </a>
          </nav>
        </div>

        <div class="flex items-center gap-2">
          <div class="px-2 py-1 rounded bg-zinc-100 text-xs font-mono font-medium text-zinc-700 flex items-center gap-1.5 border border-zinc-200/60">
            <span>{{ user?.username || 'admin' }}</span>
            <span class="text-[10px] px-1 rounded font-semibold bg-zinc-900 text-white">ADMIN</span>
          </div>
          <button
            @click="handleLogout"
            class="p-1.5 text-zinc-400 hover:text-zinc-900 rounded-md hover:bg-zinc-100 transition-colors cursor-pointer"
            title="退出登录"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"></path>
            </svg>
          </button>
        </div>
      </div>
    </header>

    <!-- 主管理视图区域 -->
    <main class="flex-1 max-w-5xl w-full mx-auto px-4 sm:px-6 py-6">
      <AdminSandboxesView
        v-if="activeTab === 'sandboxes'"
        @show-toast="addToast"
      />
      <AdminUsersView
        v-else-if="activeTab === 'users'"
        :current-user="user"
        @show-toast="addToast"
      />
      <AdminAuditView
        v-else-if="activeTab === 'audit'"
        @show-toast="addToast"
      />
      <AdminMetricsView
        v-else-if="activeTab === 'nodes'"
        @show-toast="addToast"
      />
      <AdminSettingsView
        v-else-if="activeTab === 'settings'"
        :broadcast="broadcast"
        @show-toast="addToast"
        @broadcast-updated="broadcast = $event"
      />
    </main>

    <!-- 全局消息提示 -->
    <Toast :toasts="toasts" />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { api } from './api.js'
import Toast from './components/Toast.vue'
import AdminSandboxesView from './views/AdminSandboxesView.vue'
import AdminUsersView from './views/AdminUsersView.vue'
import AdminAuditView from './views/AdminAuditView.vue'
import AdminMetricsView from './views/AdminMetricsView.vue'
import AdminSettingsView from './views/AdminSettingsView.vue'

const user = ref(null)
const broadcast = ref({ enabled: false, message: '' })

const validTabs = ['sandboxes', 'users', 'audit', 'nodes', 'settings']
const initialTab = new URLSearchParams(window.location.search).get('tab')
const activeTab = ref(validTabs.includes(initialTab) ? initialTab : 'sandboxes')
const toasts = ref([])

let toastSeq = 0
function addToast(message) {
  const id = ++toastSeq
  toasts.value.push({ id, message })
  setTimeout(() => {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }, 2500)
}

function switchTab(tab) {
  activeTab.value = tab
  const url = new URL(window.location.href)
  url.searchParams.set('tab', tab)
  if (window.location.search !== url.search) {
    window.history.pushState(null, '', url.pathname + url.search)
  }
}

function handlePopState() {
  const tab = new URLSearchParams(window.location.search).get('tab')
  if (validTabs.includes(tab)) {
    activeTab.value = tab
  } else {
    activeTab.value = 'sandboxes'
  }
}

async function checkAdminAuth() {
  try {
    const data = await api.me()
    if (!data.loggedIn || !data.user || data.user.role !== 'admin') {
      window.location.href = '/'
      return
    }
    user.value = data.user
    if (data.broadcast) {
      broadcast.value = data.broadcast
    }
  } catch (e) {
    window.location.href = '/'
  }
}

async function handleLogout() {
  try {
    await api.logout()
  } catch (e) {}
  window.location.href = '/'
}

onMounted(() => {
  window.addEventListener('popstate', handlePopState)
  checkAdminAuth()
})

onUnmounted(() => {
  window.removeEventListener('popstate', handlePopState)
})
</script>
