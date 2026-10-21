<template>
  <header class="bg-white border-b border-zinc-200 sticky top-0 z-30">
    <div class="max-w-5xl mx-auto px-4 sm:px-6 h-14 flex items-center justify-between gap-4">
      <div class="flex items-center gap-6">
        <div class="flex items-center gap-2">
          <img src="/favicon.ico" alt="AttackBox Logo" class="w-8 h-8 rounded-md object-contain" />
          <span class="font-semibold text-sm tracking-tight text-zinc-900 font-mono">AttackBox</span>
        </div>

        <nav class="hidden sm:flex items-center gap-1">
          <!-- 工作区 (所有用户) -->
          <button
            @click="$emit('update:activeTab', 'workspace')"
            :class="[
              'px-2.5 py-1.5 rounded-md text-xs font-mono font-medium transition-all cursor-pointer',
              activeTab === 'workspace'
                ? 'bg-zinc-900 text-white shadow-sm'
                : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900'
            ]"
          >
            WORKSPACE
          </button>

          <!-- 个人账户 (所有用户) -->
          <button
            @click="$emit('update:activeTab', 'account')"
            :class="[
              'px-2.5 py-1.5 rounded-md text-xs font-mono font-medium transition-all cursor-pointer',
              activeTab === 'account'
                ? 'bg-zinc-900 text-white shadow-sm'
                : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900'
            ]"
          >
            ACCOUNT
          </button>

          <!-- 管理员入口 (仅对管理员显示跳转，普通用户完全隐藏) -->
          <a
            v-if="isAdmin"
            href="/admin"
            class="px-2.5 py-1.5 rounded-md text-xs font-mono font-medium text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 transition-all cursor-pointer flex items-center gap-1"
            title="进入独立管理控制台"
          >
            <span>ADMIN</span>
            <svg class="w-3 h-3 text-zinc-400" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"></path>
            </svg>
          </a>
        </nav>
      </div>

      <div class="flex items-center gap-2">
        <div class="px-2 py-1 rounded bg-zinc-100 text-xs font-mono font-medium text-zinc-700 flex items-center gap-1.5 border border-zinc-200/60">
          <span>{{ user?.username || 'user' }}</span>
          <span
            :class="[
              'text-[10px] px-1 rounded font-semibold',
              isAdmin ? 'bg-zinc-900 text-white' : 'bg-zinc-700 text-white'
            ]"
          >
            {{ isAdmin ? 'ADMIN' : 'USER' }}
          </span>
        </div>
        <button
          @click="$emit('logout')"
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
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  activeTab: {
    type: String,
    default: 'workspace',
  },
  user: {
    type: Object,
    default: () => ({}),
  },
})

defineEmits(['update:activeTab', 'logout'])

const isAdmin = computed(() => props.user?.role === 'admin')
</script>
