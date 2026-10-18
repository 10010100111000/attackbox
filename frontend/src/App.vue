<template>
  <!-- 独立视图: CLI 虚拟终端空间 -->
  <CliView v-if="currentPath === '/term'" />

  <!-- 独立视图: GUI 远程桌面 -->
  <GuiView v-else-if="currentPath === '/gui'" />

  <!-- 主控制台视图体系 (仅用户端，完全解耦移除管理组件) -->
  <div v-else class="min-h-full flex flex-col flex-1">
    <!-- 未登录视图 -->
    <AuthView
      v-if="!user"
      @auth-success="onAuthSuccess"
      @show-toast="addToast"
    />

    <!-- 已登录控制台视图 -->
    <div v-else class="flex-1 flex flex-col min-h-screen">
      <!-- 顶部全站公告 -->
      <BroadcastBanner
        :show="broadcast.enabled && !bannerDismissed && !!broadcast.message"
        :message="broadcast.message"
        @dismiss="bannerDismissed = true"
      />

      <!-- 顶部导航 -->
      <Navbar
        :user="user"
        :active-tab="activeTab"
        @update:active-tab="handleTabChange"
        @logout="handleLogout"
      />

      <!-- 主视图区域 (普通用户端仅包含工作区与个人中心) -->
      <main class="flex-1 max-w-5xl w-full mx-auto px-4 sm:px-6 py-6">
        <!-- 1. 工作区 (默认) -->
        <WorkspaceView
          v-if="activeTab === 'workspace'"
          :box-id="boxId"
          :is-running="isRunning"
          :expires-in="expiresIn"
          @show-toast="addToast"
          @env-updated="checkAuth"
        />

        <!-- 2. 个人安全凭证与设置 -->
        <AccountView
          v-else-if="activeTab === 'account'"
          :user="user"
          @show-toast="addToast"
        />
      </main>
    </div>

    <!-- 全局消息提示 -->
    <Toast :toasts="toasts" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { api } from './api.js'
import Navbar from './components/Navbar.vue'
import BroadcastBanner from './components/BroadcastBanner.vue'
import Toast from './components/Toast.vue'
import AuthView from './views/AuthView.vue'
import WorkspaceView from './views/WorkspaceView.vue'
import AccountView from './views/AccountView.vue'
import CliView from './views/CliView.vue'
import GuiView from './views/GuiView.vue'

const currentPath = ref(window.location.pathname)

const user = ref(null)
const boxId = ref('')
const cliRunning = ref(false)
const guiRunning = ref(false)
const expiresIn = ref(0)
const isRunning = computed(() => cliRunning.value || guiRunning.value)

const broadcast = ref({ enabled: false, message: '' })
const bannerDismissed = ref(false)

const activeTab = ref(window.location.pathname === '/account' ? 'account' : 'workspace')
const toasts = ref([])

let toastSeq = 0
function addToast(message) {
  const id = ++toastSeq
  toasts.value.push({ id, message })
  setTimeout(() => {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }, 2500)
}

function handleTabChange(tab) {
  activeTab.value = tab === 'account' ? 'account' : 'workspace'
  const targetPath = activeTab.value === 'account' ? '/account' : '/workspace'
  if (window.location.pathname !== targetPath) {
    window.history.pushState(null, '', targetPath)
  }
}

function handlePopState() {
  currentPath.value = window.location.pathname
  if (currentPath.value === '/account') {
    activeTab.value = 'account'
  } else if (currentPath.value === '/workspace' || currentPath.value === '/') {
    activeTab.value = 'workspace'
  }
}

async function checkAuth() {
  if (currentPath.value === '/term' || currentPath.value === '/gui') {
    return
  }
  try {
    const data = await api.me()
    if (data.loggedIn && data.user) {
      user.value = data.user
      boxId.value = data.boxId || ''
      cliRunning.value = !!data.cliRunning
      guiRunning.value = !!data.guiRunning
      expiresIn.value = typeof data.expiresIn === 'number' ? data.expiresIn : 0
      if (data.broadcast) {
        broadcast.value = data.broadcast
      }

      // 登录状态下更新地址栏
      if (window.location.pathname === '/login' || window.location.pathname === '/') {
        activeTab.value = 'workspace'
        window.history.replaceState(null, '', '/workspace')
      } else if (window.location.pathname === '/account') {
        activeTab.value = 'account'
      } else if (window.location.pathname === '/workspace') {
        activeTab.value = 'workspace'
      }
    } else {
      user.value = null
      activeTab.value = 'workspace'
      if (window.location.pathname !== '/login') {
        window.history.replaceState(null, '', '/login')
      }
    }
  } catch (e) {
    user.value = null
    activeTab.value = 'workspace'
    if (window.location.pathname !== '/login') {
      window.history.replaceState(null, '', '/login')
    }
  }
}

function onAuthSuccess(userData) {
  user.value = userData
  activeTab.value = 'workspace'
  window.history.replaceState(null, '', '/workspace')
  checkAuth()
}

async function handleLogout() {
  try {
    await api.logout()
  } catch (e) {}
  user.value = null
  activeTab.value = 'workspace'
  window.history.replaceState(null, '', '/login')
  addToast('已安全退出')
}

onMounted(() => {
  window.addEventListener('popstate', handlePopState)
  if (currentPath.value !== '/term' && currentPath.value !== '/gui') {
    checkAuth()
  }
})

onUnmounted(() => {
  window.removeEventListener('popstate', handlePopState)
})
</script>
