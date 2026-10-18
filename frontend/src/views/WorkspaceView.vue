<template>
  <section class="space-y-6">
    <!-- AttackBox HUD Strip -->
    <div class="shadcn-card px-4 py-3 sm:px-5 sm:py-3.5">
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
        <!-- 左侧: 实例状态与租约倒计时 -->
        <div class="flex items-center gap-3 flex-wrap">
          <div class="flex items-center gap-2">
            <span class="text-xs font-mono font-bold tracking-wider text-zinc-900">ATTACKBOX</span>
            <span
              :class="[
                'px-2 py-0.5 rounded text-[10px] font-mono font-semibold flex items-center gap-1.5 border transition-colors',
                isRunning
                  ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
                  : 'bg-zinc-100 text-zinc-500 border-zinc-200'
              ]"
            >
              <span
                :class="[
                  'w-1.5 h-1.5 rounded-full',
                  isRunning ? 'bg-emerald-500 animate-pulse' : 'bg-zinc-400'
                ]"
              ></span>
              <span>{{ isRunning ? (boxId ? `#${boxId} ONLINE` : 'ONLINE') : 'STANDBY' }}</span>
            </span>
          </div>

          <div class="h-3.5 w-px bg-zinc-200 hidden sm:block"></div>

          <!-- 倒计时指示器 -->
          <div class="flex items-center gap-1.5 px-2.5 py-1 rounded bg-zinc-50 border border-zinc-200/80 text-zinc-700 font-mono text-xs">
            <svg class="w-3.5 h-3.5 text-zinc-500" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <circle cx="12" cy="12" r="10"></circle>
              <polyline points="12 6 12 12 16 14"></polyline>
            </svg>
            <span class="text-[11px] text-zinc-500 font-medium">LEASE:</span>
            <span :class="['font-semibold tracking-wide', isRunning ? 'text-zinc-900' : 'text-zinc-500']">
              {{ leaseDisplay }}
            </span>
            <span v-if="isRunning" class="text-[10px] text-zinc-400 font-mono" title="10分钟无交互自动释放">(IDLE: 10m)</span>
          </div>
        </div>

        <!-- 右侧: 对称操作按钮群 -->
        <div class="flex items-center gap-2 flex-wrap">
          <!-- CLI 终端按钮 -->
          <button
            @click="handleOpenCli"
            class="px-3 py-1.5 rounded-md text-xs font-mono font-semibold bg-zinc-900 text-white hover:bg-zinc-800 shadow-sm flex items-center gap-1.5 transition-all cursor-pointer active:scale-95"
            title="在新窗口打开 CLI 虚拟终端空间"
          >
            <svg class="w-3.5 h-3.5 text-zinc-300" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <polyline points="4 17 10 11 4 5"></polyline>
              <line x1="12" y1="19" x2="20" y2="19"></line>
            </svg>
            <span>TERMINAL</span>
          </button>

          <!-- GUI 桌面按钮 -->
          <button
            @click="handleOpenGui"
            class="px-3 py-1.5 rounded-md text-xs font-mono font-semibold bg-zinc-900 text-white hover:bg-zinc-800 shadow-sm flex items-center gap-1.5 transition-all cursor-pointer active:scale-95"
            title="在新窗口打开 GUI 远程桌面"
          >
            <svg class="w-3.5 h-3.5 text-zinc-300" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <rect x="2" y="3" width="20" height="14" rx="2"></rect>
              <line x1="8" y1="21" x2="16" y2="21"></line>
              <line x1="12" y1="17" x2="12" y2="21"></line>
            </svg>
            <span>DESKTOP</span>
          </button>

          <!-- 释放沙箱按钮 (运行中展示) -->
          <button
            v-if="isRunning"
            @click="handleDestroy"
            :disabled="destroying"
            class="px-2.5 py-1.5 rounded-md text-xs font-mono font-medium text-zinc-600 hover:text-zinc-900 bg-zinc-100 hover:bg-zinc-200 border border-zinc-200 flex items-center gap-1.5 transition-all disabled:opacity-50 cursor-pointer active:scale-95"
            title="释放沙箱"
          >
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
            </svg>
            <span>{{ destroying ? 'RELEASING...' : 'RELEASE' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- TARGETS 靶机列表区 -->
    <div class="space-y-3">
      <div class="flex items-center justify-between pb-1 border-b border-zinc-200/60">
        <h3 class="text-xs font-mono font-bold tracking-wider text-zinc-900">TARGETS</h3>
        <span class="text-[11px] font-mono text-zinc-400">ISOLATED SUBNET · DIRECT ROUTING</span>
      </div>

      <!-- 靶机卡片网格 -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div class="shadcn-card p-4 flex flex-col justify-between hover:border-zinc-300 transition-colors">
          <div>
            <div class="flex items-start justify-between gap-2">
              <div>
                <h4 class="text-xs font-semibold text-zinc-900">DVWA 综合 Web 漏洞靶机</h4>
                <div class="flex items-center gap-1.5 mt-1.5 flex-wrap">
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">LINUX</span>
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">WEB</span>
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">DOCKER</span>
                </div>
              </div>
              <span class="px-2 py-0.5 rounded text-[10px] font-mono font-medium bg-zinc-100 text-zinc-700 border border-zinc-200">
                ACTIVE
              </span>
            </div>
            <p class="text-xs text-zinc-500 mt-2.5 leading-relaxed">包含 SQL 注入、XSS、文件上传与命令执行等常见 Web 漏洞环境。</p>
          </div>
          <div class="mt-4 pt-3 border-t border-zinc-100 flex items-center justify-between gap-2">
            <div class="text-xs font-mono flex items-center gap-1.5">
              <span class="text-zinc-400">IP:</span>
              <span class="font-semibold text-zinc-900">172.17.0.2</span>
            </div>
            <button
              @click="copyIp('172.17.0.2')"
              class="px-2 py-0.5 text-[11px] font-mono rounded bg-zinc-100 hover:bg-zinc-200 text-zinc-700 transition-colors cursor-pointer"
            >
              COPY
            </button>
          </div>
        </div>

        <div class="shadcn-card p-4 flex flex-col justify-between hover:border-zinc-300 transition-colors">
          <div>
            <div class="flex items-start justify-between gap-2">
              <div>
                <h4 class="text-xs font-semibold text-zinc-900">Struts2 历史漏洞镜像</h4>
                <div class="flex items-center gap-1.5 mt-1.5 flex-wrap">
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">LINUX</span>
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">RCE</span>
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">LAB-NET</span>
                </div>
              </div>
              <span class="px-2 py-0.5 rounded text-[10px] font-mono font-medium bg-zinc-100 text-zinc-500 border border-zinc-200">
                REACHABLE
              </span>
            </div>
            <p class="text-xs text-zinc-500 mt-2.5 leading-relaxed">用于复现 S2-045、S2-057 等远程代码执行漏洞的标准测试靶场。</p>
          </div>
          <div class="mt-4 pt-3 border-t border-zinc-100 flex items-center justify-between gap-2">
            <div class="text-xs font-mono flex items-center gap-1.5">
              <span class="text-zinc-400">IP:</span>
              <span class="font-semibold text-zinc-900">10.10.10.25</span>
            </div>
            <button
              @click="copyIp('10.10.10.25')"
              class="px-2 py-0.5 text-[11px] font-mono rounded bg-zinc-100 hover:bg-zinc-200 text-zinc-700 transition-colors cursor-pointer"
            >
              COPY
            </button>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { api } from '../api.js'

const props = defineProps({
  boxId: String,
  isRunning: Boolean,
  expiresIn: {
    type: Number,
    default: 0
  }
})

const emit = defineEmits(['show-toast', 'env-updated'])

const destroying = ref(false)
const remainingSeconds = ref(props.expiresIn || 0)
let timer = null

watch(() => props.expiresIn, (newVal) => {
  remainingSeconds.value = typeof newVal === 'number' ? newVal : 0
})

onMounted(() => {
  timer = setInterval(() => {
    if (props.isRunning && remainingSeconds.value > 0) {
      remainingSeconds.value--
      if (remainingSeconds.value === 0) {
        emit('env-updated')
      }
    }
  }, 1000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

const leaseDisplay = computed(() => {
  if (!props.isRunning || remainingSeconds.value <= 0) {
    return 'STANDBY'
  }
  const total = remainingSeconds.value
  const h = String(Math.floor(total / 3600)).padStart(2, '0')
  const m = String(Math.floor((total % 3600) / 60)).padStart(2, '0')
  const s = String(total % 60).padStart(2, '0')
  return `${h}:${m}:${s}`
})

async function handleOpenCli() {
  if (!props.isRunning) {
    try {
      await api.toggleCli()
      emit('env-updated')
    } catch (e) {}
  }
  window.open('/term', '_blank')
  emit('show-toast', '正在打开 TERMINAL 空间...')
}

async function handleOpenGui() {
  if (!props.isRunning) {
    try {
      await api.toggleGui()
      emit('env-updated')
    } catch (e) {}
  }
  window.open('/gui', '_blank')
  emit('show-toast', '正在打开 DESKTOP 桌面...')
}

async function handleDestroy() {
  if (!confirm('确定要彻底释放当前渗透沙箱吗？未保存的数据将被清空，所有正在运行的会话将被终止。')) {
    return
  }
  destroying.value = true
  try {
    const data = await api.destroyEnv()
    if (!data.success) {
      emit('show-toast', data.error || '释放失败')
      return
    }
    emit('show-toast', '沙箱环境已安全销毁释放')
    emit('env-updated')
  } catch (e) {
    emit('show-toast', '释放异常: ' + e.message)
  } finally {
    destroying.value = false
  }
}

function copyIp(ip) {
  navigator.clipboard.writeText(ip).then(
    () => emit('show-toast', '已复制: ' + ip),
    () => emit('show-toast', '复制失败，请手动复制')
  )
}
</script>
