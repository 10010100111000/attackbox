<script setup lang="ts">
const confirmAction = useConfirm()
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { ArrowLeft, RefreshCw, ShieldCheck, BookOpen, Monitor } from 'lucide-vue-next'
import { userStatusLabels, instanceStateLabels, type AdminUserDetail } from '~/shared/admin-users'
import { useAuth } from '~/composables/useAuth'
definePageMeta({ layout: 'admin' })
useHead({ title: '用户详情 — AttackBox' })
const route = useRoute(), { currentUser } = useAuth()
const id = String(route.params.id)
const { data, pending, error, refresh } = await useLazyFetch<{ data: AdminUserDetail }>(`/api/admin/users/${id}`, { key: `admin:user:${id}` })
const detail = computed(() => data.value?.data), busy = ref(false), notice = ref(''), clock = ref(Date.now())
const self = computed(() => detail.value?.user.id === currentUser.value?.id)
const runtime = computed(() => {
  if (!detail.value?.instance) return ''
  const minutes = Math.max(0, Math.floor((clock.value - Date.parse(detail.value.instance.startedAt)) / 60000))
  return minutes < 60 ? `${minutes} 分钟` : `${Math.floor(minutes / 60)} 小时 ${minutes % 60} 分钟`
})
function date(value: string | null) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—' }
async function changeStatus() {
  if (!detail.value || busy.value) return
  const suspended = detail.value.user.status !== 'suspended'
  if (!(await confirmAction({title:suspended?'停用账号？':'恢复账号？',description:suspended?'撤销该账号的全部网页登录并释放当前环境。':'未验证邮箱的账号仍需完成验证。',confirmLabel:suspended?'停用账号':'恢复账号',tone:suspended?'danger':'neutral'}))) return
  busy.value = true; notice.value = ''
  try {
    const result = await $fetch<{ data: { cleanup: string } }>(`/api/admin/users/${id}/status`, { method: 'PUT', body: { suspended } })
    notice.value = result.data.cleanup === 'pending' ? '账号已停用；环境清理待确认，可在运行实例中重试。' : suspended ? '账号已停用。' : '账号已恢复。'
  } catch (e: any) { notice.value = e?.data?.message || '操作失败，请重试。' }
  finally { busy.value = false; await refresh() }
}
async function revoke() {
  if (busy.value || !(await confirmAction({title:'撤销全部网页登录？',description:'当前环境和已连接的终端会保留。如需关闭，请释放环境。',confirmLabel:'撤销登录'}))) return
  busy.value = true; notice.value = ''
  try { await $fetch(`/api/admin/users/${id}/revoke-sessions`, { method: 'POST' }); notice.value = '全部网页登录已撤销。' }
  catch (e: any) { notice.value = e?.data?.message || '操作失败，请重试。' }
  finally { busy.value = false; await refresh() }
}
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => { timer = setInterval(() => { clock.value = Date.now(); if (!document.hidden && !busy.value) refresh() }, 15000) })
onBeforeUnmount(() => clearInterval(timer))
</script>
<template>
  <div class="users-page">
    <NuxtLink to="/admin/users" class="back"><ArrowLeft :size="14" aria-hidden="true" />用户列表</NuxtLink>
    <AdminDataLoading :pending="pending" :has-data="!!data" />
    <p v-if="error" class="notice" role="alert">无法读取用户详情。用户可能不存在，或当前登录已失效。</p>
    <template v-else-if="detail">
      <header class="page-heading"><div><div class="title-line"><h1>{{ detail.user.displayName }}</h1><span class="status" :class="detail.user.status">{{ userStatusLabels[detail.user.status] }}</span></div><p>{{ detail.user.email }}</p></div><button :disabled="pending || busy" @click="refresh()"><RefreshCw :size="14" aria-hidden="true" />刷新</button></header>
      <p v-if="notice" class="notice" role="status">{{ notice }}</p>
      <div class="detail-grid">
        <section class="panel account"><h2><ShieldCheck :size="17" aria-hidden="true" />账号</h2><dl><dt>角色</dt><dd>{{ detail.user.role === 'admin' ? '管理员' : '用户' }}<span v-if="self" class="muted">（当前账号）</span></dd><dt>注册时间</dt><dd>{{ date(detail.user.createdAt) }}</dd><dt>邮箱验证</dt><dd>{{ date(detail.user.emailVerifiedAt) }}</dd><dt>最近登录</dt><dd>{{ date(detail.user.lastLoginAt) }}</dd><dt>有效网页登录</dt><dd>{{ detail.activeSessions }} 个</dd></dl><div class="account-actions"><button :disabled="busy || self" :title="self ? '不能停用当前管理员' : undefined" :class="{ danger: detail.user.status !== 'suspended' }" @click="changeStatus">{{ detail.user.status === 'suspended' ? '恢复账号' : '停用账号' }}</button><button :disabled="busy || self || !detail.activeSessions" :title="self ? '请使用侧栏退出当前账号' : undefined" @click="revoke">撤销登录</button></div></section>
        <section class="panel environment"><h2><Monitor :size="17" aria-hidden="true" />当前环境<NuxtLink to="/admin/instances" class="section-link">管理实例</NuxtLink></h2><template v-if="detail.instance"><div class="title-line"><span class="status">{{ instanceStateLabels[detail.instance.state] }}</span><span v-if="!detail.instance.connected" class="muted">节点断开</span></div><dl><dt>节点</dt><dd>{{ detail.instance.nodeId }}</dd><dt>本次运行</dt><dd>{{ runtime }}</dd><dt>启动时间</dt><dd>{{ date(detail.instance.startedAt) }}</dd><dt>到期时间</dt><dd>{{ date(detail.instance.expiresAt) }}</dd></dl><p v-if="detail.instance.error" class="notice">{{ detail.instance.error }}</p><code>{{ detail.instance.id }}</code></template><div v-else class="empty compact"><Monitor :size="24" aria-hidden="true" /><p>当前没有运行中的环境</p></div><p class="footnote">显示本次环境运行时长，尚未统计历史 CLI / GUI 使用时长。</p></section>
        <section class="panel learning"><h2><BookOpen :size="17" aria-hidden="true" />学习记录</h2><div class="metrics"><div><strong>{{ detail.progress.modules }}</strong><span>已学习模块</span></div><div><strong>{{ detail.progress.visited }}</strong><span>已访问章节</span></div><div><strong>{{ detail.progress.completed }}</strong><span>已完成章节</span></div></div><div v-if="detail.history.length" class="history"><div v-for="item in detail.history" :key="item.moduleId" class="history-row"><div><NuxtLink :to="`/learn/modules/${item.slug}`" target="_blank">{{ item.title }}</NuxtLink><span class="secondary">学习章节：{{ item.chapterTitle }}</span></div><div class="progress"><progress :value="item.progress.completedChapters" :max="item.progress.totalChapters || 1" :aria-label="`${item.title}学习进度`" /><span>{{ item.progress.completedChapters }} / {{ item.progress.totalChapters }}</span></div><time>{{ date(item.lastVisitedAt) }}</time></div></div><p v-else class="empty compact">暂无学习记录</p><p class="footnote">统计当前已发布模块；最近记录最多显示 20 个模块。</p></section>
      </div>
    </template><UiBusyIndicator v-else label="用户资料加载"/>
  </div>
</template>
<style scoped src="~/assets/css/admin-users.css"></style>
