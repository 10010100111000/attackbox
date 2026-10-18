<template>
  <section class="space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-base font-semibold text-zinc-900">用户与访问管理</h2>
        <p class="text-xs text-zinc-500 mt-0.5">管理平台用户账号与权限；禁用账号将自动注销其 Token 并连根回收沙箱</p>
      </div>
    </div>

    <!-- 数据表格 -->
    <DataTable
      :data="users"
      :columns="columns"
      search-placeholder="过滤用户名或角色..."
      :page-size="10"
    >
      <template #actions>
        <button
          @click="fetchUsers"
          :disabled="loading"
          class="px-2.5 py-1.5 rounded-lg text-xs font-medium shadcn-btn-outline flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
          title="刷新用户列表"
        >
          <RotateCcw :class="['w-3.5 h-3.5', loading ? 'animate-spin' : '']" />
          <span>刷新</span>
        </button>

        <button
          @click="showAddModal = true"
          class="px-3 py-1.5 rounded-lg text-xs font-medium shadcn-btn-primary flex items-center gap-1.5 cursor-pointer"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>添加用户</span>
        </button>
      </template>

      <!-- 用户名列自定义渲染 -->
      <template #cell-username="{ value }">
        <span class="font-mono font-medium text-zinc-900">{{ value }}</span>
      </template>

      <!-- 角色列 -->
      <template #cell-role="{ value }">
        <span
          :class="[
            'text-[10px] px-1.5 py-0.5 rounded font-medium inline-block',
            value === 'admin'
              ? 'bg-zinc-900 text-white'
              : 'bg-zinc-100 text-zinc-700 border border-zinc-200'
          ]"
        >
          {{ value === 'admin' ? '管理员' : '普通用户' }}
        </span>
      </template>

      <!-- 状态列 -->
      <template #cell-active="{ row }">
        <button
          @click="toggleActive(row)"
          :disabled="currentUser?.username === row.username"
          :class="[
            'px-2 py-0.5 rounded-full text-[11px] font-medium border flex items-center transition-all',
            row.active
              ? 'bg-zinc-900 text-zinc-100 border-zinc-700 hover:bg-zinc-800'
              : 'bg-zinc-100 text-zinc-500 border-zinc-200 hover:bg-zinc-200',
            currentUser?.username === row.username ? 'opacity-60 cursor-not-allowed' : 'cursor-pointer'
          ]"
          :title="currentUser?.username === row.username ? '不能禁用当前登录账号' : '点击切换启用/禁用状态'"
        >
          <span
            :class="[
              'w-1.5 h-1.5 rounded-full mr-1.5',
              row.active ? 'bg-zinc-100' : 'bg-zinc-400'
            ]"
          ></span>
          <span>{{ row.active ? '已启用' : '已禁用' }}</span>
        </button>
      </template>

      <!-- 创建时间列 -->
      <template #cell-created_at="{ value }">
        <span class="text-zinc-400 font-mono text-[11px]">{{ formatTime(value) }}</span>
      </template>

      <!-- 操作列 -->
      <template #cell-actions="{ row }">
        <div class="space-x-2">
          <button
            @click="openResetPass(row)"
            class="text-zinc-600 hover:text-zinc-900 underline text-[11px] cursor-pointer"
          >
            重置密码
          </button>
          <button
            @click="deleteUser(row)"
            :disabled="currentUser?.username === row.username"
            :class="[
              'text-red-500 hover:text-red-700 underline text-[11px] cursor-pointer',
              currentUser?.username === row.username ? 'opacity-40 cursor-not-allowed' : ''
            ]"
            :title="currentUser?.username === row.username ? '不能删除当前登录账号' : '删除用户'"
          >
            删除
          </button>
        </div>
      </template>
    </DataTable>

    <!-- 弹窗：新建用户 -->
    <Modal v-model="showAddModal" title="添加新用户">
      <form @submit.prevent="submitAddUser" class="space-y-3.5 text-xs">
        <div>
          <label class="block font-medium text-zinc-700 mb-1">用户名</label>
          <input
            v-model.trim="newUserName"
            type="text"
            required
            placeholder="3-32位字母数字下划线"
            class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900"
          />
        </div>
        <div>
          <label class="block font-medium text-zinc-700 mb-1">初始密码</label>
          <input
            v-model="newUserPass"
            type="password"
            required
            placeholder="至少 6 位密码"
            class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900"
          />
        </div>
        <div>
          <label class="block font-medium text-zinc-700 mb-1">角色权限</label>
          <select
            v-model="newUserRole"
            class="w-full px-3 py-2 border border-zinc-200 rounded-lg bg-white"
          >
            <option value="user">普通用户 (User)</option>
            <option value="admin">管理员 (Admin)</option>
          </select>
        </div>
        <div class="pt-2 flex justify-end gap-2">
          <button
            type="button"
            @click="showAddModal = false"
            class="px-3 py-1.5 rounded-lg shadcn-btn-outline cursor-pointer"
          >
            取消
          </button>
          <button
            type="submit"
            :disabled="addingUser"
            class="px-3.5 py-1.5 rounded-lg shadcn-btn-primary disabled:opacity-50 cursor-pointer"
          >
            {{ addingUser ? '添加中...' : '确认添加' }}
          </button>
        </div>
      </form>
    </Modal>

    <!-- 弹窗：重置密码 -->
    <Modal v-model="showResetModal" :title="`重置用户【${targetUser}】密码`">
      <form @submit.prevent="submitResetPass" class="space-y-3.5 text-xs">
        <div>
          <label class="block font-medium text-zinc-700 mb-1">新密码</label>
          <input
            v-model="resetPass"
            type="password"
            required
            placeholder="输入新密码"
            class="w-full px-3 py-2 border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900"
          />
        </div>
        <div class="pt-2 flex justify-end gap-2">
          <button
            type="button"
            @click="showResetModal = false"
            class="px-3 py-1.5 rounded-lg shadcn-btn-outline cursor-pointer"
          >
            取消
          </button>
          <button
            type="submit"
            :disabled="resettingPass"
            class="px-3.5 py-1.5 rounded-lg shadcn-btn-primary disabled:opacity-50 cursor-pointer"
          >
            {{ resettingPass ? '保存中...' : '保存密码' }}
          </button>
        </div>
      </form>
    </Modal>
  </section>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { adminApi as api } from '../adminApi.js'
import DataTable from '../components/DataTable.vue'
import Modal from '../components/Modal.vue'
import { Plus, RotateCcw } from 'lucide-vue-next'

defineProps({
  currentUser: Object,
})

const emit = defineEmits(['show-toast'])

const users = ref([])
const loading = ref(false)

// 列配置
const columns = [
  {
    accessorKey: 'username',
    header: '用户名',
    enableSorting: true,
  },
  {
    accessorKey: 'role',
    header: '角色',
    enableSorting: true,
  },
  {
    accessorKey: 'active',
    header: '状态',
    enableSorting: true,
  },
  {
    accessorKey: 'created_at',
    header: '创建时间',
    enableSorting: true,
  },
  {
    id: 'actions',
    header: '操作',
    enableSorting: false,
    meta: { align: 'right' },
  },
]

// 新增用户状态
const showAddModal = ref(false)
const newUserName = ref('')
const newUserPass = ref('')
const newUserRole = ref('user')
const addingUser = ref(false)

// 重置密码状态
const showResetModal = ref(false)
const targetUser = ref('')
const resetPass = ref('')
const resettingPass = ref(false)

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

async function fetchUsers() {
  loading.value = true
  try {
    const data = await api.getUsers()
    users.value = data || []
  } catch (e) {
    emit('show-toast', '获取用户列表失败')
  } finally {
    loading.value = false
  }
}

async function submitAddUser() {
  addingUser.value = true
  try {
    const data = await api.createUser(newUserName.value, newUserPass.value, newUserRole.value)
    if (!data.success) {
      emit('show-toast', data.error || '创建用户失败')
      return
    }
    showAddModal.value = false
    emit('show-toast', `已成功创建用户: ${newUserName.value}`)
    newUserName.value = ''
    newUserPass.value = ''
    fetchUsers()
  } catch (e) {
    emit('show-toast', '创建用户失败: ' + e.message)
  } finally {
    addingUser.value = false
  }
}

function openResetPass(u) {
  targetUser.value = u.username
  resetPass.value = ''
  showResetModal.value = true
}

async function submitResetPass() {
  resettingPass.value = true
  try {
    const data = await api.resetPassword(targetUser.value, resetPass.value)
    if (!data.success) {
      emit('show-toast', data.error || '重置失败')
      return
    }
    showResetModal.value = false
    emit('show-toast', `已重置【${targetUser.value}】的密码`)
  } catch (e) {
    emit('show-toast', '重置失败: ' + e.message)
  } finally {
    resettingPass.value = false
  }
}

async function toggleActive(u) {
  try {
    const data = await api.toggleUser(u.username)
    if (!data.success) {
      emit('show-toast', data.error || '切换失败')
      return
    }
    emit('show-toast', '用户状态已更新')
    fetchUsers()
  } catch (e) {
    emit('show-toast', '切换失败: ' + e.message)
  }
}

async function deleteUser(u) {
  if (!confirm(`确定要彻底删除用户【${u.username}】吗？`)) return
  try {
    const data = await api.deleteUser(u.username)
    if (!data.success) {
      emit('show-toast', data.error || '删除失败')
      return
    }
    emit('show-toast', `用户【${u.username}】已删除`)
    fetchUsers()
  } catch (e) {
    emit('show-toast', '删除失败: ' + e.message)
  }
}

onMounted(fetchUsers)
</script>
