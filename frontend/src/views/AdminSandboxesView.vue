<template>
  <section class="space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-base font-semibold text-zinc-900">沙箱与容量调度</h2>
        <p class="text-xs text-zinc-500 mt-0.5">实时监控运行中的靶场容器；【回收资源】仅释放 VPS 内存，用户登录态不受影响</p>
      </div>
    </div>

    <!-- 数据表格 -->
    <DataTable
      :data="sandboxes"
      :columns="columns"
      search-placeholder="过滤使用者或沙箱 ID..."
      :page-size="10"
    >
      <template #actions>
        <button
          @click="fetchSandboxes"
          :disabled="loading"
          class="px-2.5 py-1.5 rounded-lg text-xs font-medium shadcn-btn-outline flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
          title="刷新沙箱状态"
        >
          <RotateCcw :class="['w-3.5 h-3.5', loading ? 'animate-spin' : '']" />
          <span>刷新</span>
        </button>
      </template>

      <!-- 使用者列 -->
      <template #cell-user="{ value }">
        <span class="font-mono font-medium text-zinc-900">{{ value }}</span>
      </template>

      <!-- 沙箱标识列 -->
      <template #cell-box_id="{ value }">
        <span class="font-mono text-zinc-600 bg-zinc-100 px-1.5 py-0.5 rounded text-[11px] border border-zinc-200">
          {{ value }}
        </span>
      </template>

      <!-- 状态列 -->
      <template #cell-running="{ value }">
        <span
          :class="[
            'text-[10px] px-2 py-0.5 rounded-full font-medium inline-flex items-center gap-1 border',
            value
              ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
              : 'bg-zinc-100 text-zinc-600 border-zinc-200'
          ]"
        >
          <span :class="['w-1.5 h-1.5 rounded-full', value ? 'bg-emerald-500' : 'bg-zinc-400']"></span>
          <span>{{ value ? '运行中' : '已就绪' }}</span>
        </span>
      </template>

      <!-- 启动时间列 -->
      <template #cell-started_at="{ value }">
        <span class="text-zinc-500 font-mono text-[11px]">{{ formatTime(value) }}</span>
      </template>

      <!-- 剩余租期倒计时列 -->
      <template #cell-ttl_remaining_sec="{ value }">
        <span :class="['font-mono text-[11px]', value < 600 ? 'text-amber-600 font-semibold' : 'text-zinc-600']">
          {{ formatRemaining(value) }}
        </span>
      </template>

      <!-- 操作列 -->
      <template #cell-actions="{ row }">
        <button
          @click="releaseBox(row)"
          :disabled="releasingUser === row.user"
          class="text-red-600 hover:text-red-800 text-[11px] underline disabled:opacity-40 cursor-pointer"
          title="强制终止该容器以回收宿主机内存"
        >
          {{ releasingUser === row.user ? '释放中...' : '回收资源' }}
        </button>
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

const sandboxes = ref([])
const loading = ref(false)
const releasingUser = ref('')

const columns = [
  {
    accessorKey: 'user',
    header: '使用者',
    enableSorting: true,
  },
  {
    accessorKey: 'box_id',
    header: '沙箱槽位',
    enableSorting: true,
  },
  {
    accessorKey: 'running',
    header: '运行状态',
    enableSorting: true,
  },
  {
    accessorKey: 'started_at',
    header: '启动时间',
    enableSorting: true,
  },
  {
    accessorKey: 'ttl_remaining_sec',
    header: '剩余租期 (TTL)',
    enableSorting: true,
  },
  {
    id: 'actions',
    header: '操作',
    enableSorting: false,
    meta: { align: 'right' },
  },
]

function formatTime(val) {
  if (!val) return '--'
  try {
    const d = new Date(val)
    if (isNaN(d.getTime())) return val
    return d.toLocaleTimeString('zh-CN', { hour12: false })
  } catch (e) {
    return val
  }
}

function formatRemaining(seconds) {
  if (seconds <= 0) return '已到期'
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  if (m >= 60) {
    const h = Math.floor(m / 60)
    const remM = m % 60
    return `${h}小时 ${remM}分`
  }
  return `${m}分 ${s}秒`
}

async function fetchSandboxes() {
  loading.value = true
  try {
    const data = await api.getSandboxes()
    sandboxes.value = data || []
  } catch (e) {
    emit('show-toast', '获取沙箱列表失败')
  } finally {
    loading.value = false
  }
}

async function releaseBox(row) {
  if (!confirm(`确定要强制释放用户【${row.user}】的沙箱【${row.box_id}】吗？\n此操作将销毁容器并回收 VPS 内存，但保留其账号登录状态。`)) {
    return
  }
  releasingUser.value = row.user
  try {
    const data = await api.releaseSandbox(row.user, row.box_id)
    if (!data.success) {
      emit('show-toast', data.error || '释放失败')
      return
    }
    emit('show-toast', `已回收用户【${row.user}】的沙箱资源`)
    fetchSandboxes()
  } catch (e) {
    emit('show-toast', '释放失败: ' + e.message)
  } finally {
    releasingUser.value = ''
  }
}

onMounted(() => {
  fetchSandboxes()
})
</script>
