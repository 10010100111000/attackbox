<template>
  <section class="space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-base font-semibold text-zinc-900">安全审计流水</h2>
        <p class="text-xs text-zinc-500 mt-0.5">仅留存凭据异常、访问越权、管理员操作等关键事件；客户端 IP 直取 Cloudflare 边缘节点</p>
      </div>

      <!-- 事件分类快捷过滤 -->
      <div class="flex items-center gap-1.5">
        <select
          v-model="selectedEventType"
          @change="fetchAuditLogs"
          class="text-xs bg-white border border-zinc-200 rounded-lg px-2.5 py-1.5 focus:outline-none focus:ring-2 focus:ring-zinc-900 cursor-pointer"
        >
          <option value="">全部事件类型</option>
          <option value="LOGIN_FAILED">登录失败 (LOGIN_FAILED)</option>
          <option value="USER_TOGGLE">启禁账号 (USER_TOGGLE)</option>
          <option value="PASSWORD_RESET">重置密码 (PASSWORD_RESET)</option>
          <option value="USER_CREATED">创建账号 (USER_CREATED)</option>
          <option value="USER_REGISTER">用户注册 (USER_REGISTER)</option>
          <option value="SANDBOX_FORCE_RELEASED">强制释放 (SANDBOX_FORCE_RELEASED)</option>
          <option value="CONFIG_CHANGED">配置变更 (CONFIG_CHANGED)</option>
        </select>
      </div>
    </div>

    <!-- 数据表格 -->
    <DataTable
      :data="logs"
      :columns="columns"
      search-placeholder="过滤操作人、IP 或审计详情..."
      :page-size="15"
    >
      <template #actions>
        <button
          @click="fetchAuditLogs"
          :disabled="loading"
          class="px-2.5 py-1.5 rounded-lg text-xs font-medium shadcn-btn-outline flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
          title="刷新审计记录"
        >
          <RotateCcw :class="['w-3.5 h-3.5', loading ? 'animate-spin' : '']" />
          <span>刷新</span>
        </button>
      </template>

      <!-- 时间戳列 -->
      <template #cell-timestamp="{ value }">
        <span class="text-zinc-500 font-mono text-[11px]">{{ formatTime(value) }}</span>
      </template>

      <!-- 事件类型列 -->
      <template #cell-event_type="{ value }">
        <span
          :class="[
            'text-[10px] px-2 py-0.5 rounded font-mono font-medium inline-block border',
            getEventBadgeClass(value)
          ]"
        >
          {{ value }}
        </span>
      </template>

      <!-- 操作人列 -->
      <template #cell-actor="{ value }">
        <span class="font-mono text-zinc-900 font-medium">{{ value || '--' }}</span>
      </template>

      <!-- IP 列 -->
      <template #cell-ip="{ value }">
        <span class="font-mono text-zinc-600 bg-zinc-100 px-1.5 py-0.5 rounded text-[11px] border border-zinc-200">
          {{ value || '--' }}
        </span>
      </template>

      <!-- 详情列 -->
      <template #cell-details="{ value }">
        <span class="text-zinc-700 text-xs break-all">{{ value }}</span>
      </template>
    </DataTable>
  </section>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { adminApi as api } from '../adminApi.js'
import DataTable from '../components/DataTable.vue'
import { RotateCcw } from 'lucide-vue-next'

const emit = defineEmits(['show-toast'])

const logs = ref([])
const loading = ref(false)
const selectedEventType = ref('')

const columns = [
  {
    accessorKey: 'timestamp',
    header: '发生时间',
    enableSorting: true,
  },
  {
    accessorKey: 'event_type',
    header: '事件类型',
    enableSorting: true,
  },
  {
    accessorKey: 'actor',
    header: '操作人',
    enableSorting: true,
  },
  {
    accessorKey: 'ip',
    header: '客户端 IP',
    enableSorting: true,
  },
  {
    accessorKey: 'details',
    header: '审计详情',
    enableSorting: false,
  },
]

function formatTime(val) {
  if (!val) return '--'
  try {
    const d = new Date(val)
    if (isNaN(d.getTime())) return val
    return d.toLocaleString('zh-CN', { hour12: false })
  } catch (e) {
    return val
  }
}

function getEventBadgeClass(type) {
  switch (type) {
    case 'LOGIN_FAILED':
      return 'bg-red-50 text-red-700 border-red-200'
    case 'USER_TOGGLE':
      return 'bg-amber-50 text-amber-700 border-amber-200'
    case 'SANDBOX_FORCE_RELEASED':
      return 'bg-orange-50 text-orange-700 border-orange-200'
    case 'CONFIG_CHANGED':
      return 'bg-purple-50 text-purple-700 border-purple-200'
    case 'PASSWORD_RESET':
      return 'bg-blue-50 text-blue-700 border-blue-200'
    default:
      return 'bg-zinc-100 text-zinc-700 border-zinc-200'
  }
}

async function fetchAuditLogs() {
  loading.value = true
  try {
    const params = { limit: 100 }
    if (selectedEventType.value) {
      params.event_type = selectedEventType.value
    }
    const data = await api.getAuditLogs(params)
    logs.value = data.items || []
  } catch (e) {
    emit('show-toast', '获取审计日志失败')
  } finally {
    loading.value = false
  }
}

onMounted(fetchAuditLogs)
</script>
