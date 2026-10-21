<template>
  <section class="space-y-5">
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-base font-semibold text-zinc-900">VPS 运行状态与环境监控</h2>
        <p class="text-xs text-zinc-500 mt-0.5">仅管理员可见：监控主机资源负载与沙箱运行数量</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          @click="fetchMetrics"
          :disabled="loading"
          class="px-3 py-1.5 rounded-lg text-xs font-medium shadcn-btn-outline disabled:opacity-50 flex items-center gap-1.5"
        >
          <svg
            :class="['w-3.5 h-3.5', loading ? 'animate-spin' : '']"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
          </svg>
          <span>刷新指标</span>
        </button>
      </div>
    </div>

    <!-- 状态指标卡片 -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
      <div class="shadcn-card p-4">
        <div class="text-xs text-zinc-500">系统状态</div>
        <div class="text-sm font-semibold text-zinc-900 mt-1">正常运行中</div>
        <div class="text-[11px] text-zinc-400 font-mono mt-1">网桥: 10.200.0.1</div>
      </div>

      <div class="shadcn-card p-4">
        <div class="text-xs text-zinc-500">CPU 负载</div>
        <div class="text-sm font-semibold text-zinc-900 mt-1">
          {{ metrics.cpuPercent !== undefined ? `${metrics.cpuPercent}%` : '--' }}
        </div>
        <div class="w-full h-1.5 bg-zinc-100 rounded-full mt-2 overflow-hidden">
          <div
            class="bg-zinc-900 h-full transition-all duration-300"
            :style="{ width: `${Math.min(metrics.cpuPercent || 0, 100)}%` }"
          ></div>
        </div>
      </div>

      <div class="shadcn-card p-4">
        <div class="text-xs text-zinc-500">内存占用 (RAM)</div>
        <div class="text-sm font-semibold text-zinc-900 mt-1">
          {{ metrics.memUsedGB !== undefined ? `${metrics.memUsedGB} GB / ${metrics.memTotalGB} GB` : '--' }}
        </div>
        <div class="w-full h-1.5 bg-zinc-100 rounded-full mt-2 overflow-hidden">
          <div
            class="bg-zinc-900 h-full transition-all duration-300"
            :style="{ width: `${metrics.memTotalGB ? Math.min((metrics.memUsedGB / metrics.memTotalGB) * 100, 100) : 0}%` }"
          ></div>
        </div>
      </div>

      <div class="shadcn-card p-4">
        <div class="text-xs text-zinc-500">磁盘空间 (btrfs)</div>
        <div class="text-sm font-semibold text-zinc-900 mt-1">
          {{ metrics.diskUsedGB !== undefined ? `${metrics.diskUsedGB} GB / ${metrics.diskTotalGB} GB` : '--' }}
        </div>
        <div class="text-[11px] text-zinc-500 mt-1">
          运行中沙箱: {{ metrics.runningBoxes !== undefined ? `${metrics.runningBoxes} 槽位` : '--' }}
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { adminApi as api } from '../adminApi.js'

const emit = defineEmits(['show-toast'])

const metrics = ref({})
const loading = ref(false)
let timer = null

async function fetchMetrics() {
  loading.value = true
  try {
    const data = await api.getMetrics()
    metrics.value = data
  } catch (e) {
    emit('show-toast', '获取指标失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchMetrics()
  timer = setInterval(fetchMetrics, 10000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>
