<script setup lang="ts">
const confirmAction = useConfirm()
import { ref, computed } from 'vue'
import { useLazyFetch, useHead } from '#imports'
import { ExternalLink } from 'lucide-vue-next'
import type { AdminModuleListItem } from '~/shared/content'

definePageMeta({
  layout: 'admin'
})

useHead({
  title: '学习模块管理 — AttackBox'
})

type AdminModuleItem = AdminModuleListItem

const difficultyLabels = { beginner: '基础入门', intermediate: '中级进阶', advanced: '高阶实战' }
const { data: response, pending, error, refresh } = await useLazyFetch<{ ok: boolean; data: AdminModuleItem[] }>(
  '/api/admin/modules', {key:'admin:modules'}
)

const modules = computed<AdminModuleItem[]>(() => {
  return response.value?.data || []
})

const notice = ref('')
const deleting = ref('')
const filterTab = ref<'全部' | '已发布' | '未发布'>('全部')
const searchQuery = ref('')

const filteredModules = computed(() => {
  let list = modules.value

  if (filterTab.value === '已发布') {
    list = list.filter(m => m.status === 'published')
  } else if (filterTab.value === '未发布') {
    list = list.filter(m => m.status === 'unpublished')
  }

  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(m => {
      const title = m.title.toLowerCase()
      const slug = m.slug.toLowerCase()
      return title.includes(q) || slug.includes(q)
    })
  }

  return list
})

async function deleteModule(id: string) {
  if (deleting.value) return
  if (!(await confirmAction({title:'永久删除模块？',description:'模块及其全部章节、题目和学习记录将被删除，此操作无法撤销。',confirmLabel:'删除模块'}))) return
  deleting.value = id; notice.value = ''
  try { await $fetch(`/api/admin/modules/${id}`, { method: 'DELETE', body: { lockVersion: modules.value.find(m => m.id === id)?.lockVersion } }); await refresh() }
  catch (error: any) { notice.value = error?.data?.message || '删除失败' }
  finally { deleting.value = '' }
}
</script>

<template>
  <div class="admin-modules-page">
    <p v-if="notice" class="notice" role="alert">{{ notice }}</p>
    <div class="page-head">
      <div>
        <h1 class="page-title">学习模块</h1>
        <p class="page-desc">预览 Git 教程，管理发布状态与运行配置。</p>
      </div>
      <div class="module-source-actions"><a class="action-btn" href="/api/admin/content/template" download="教程模板.md">下载 MD 模板</a><NuxtLink class="btn-create" to="/admin/content">检查 Git 更新</NuxtLink></div>
    </div>
    <!-- 筛选与搜索工具条 -->
    <div class="filter-bar">
      <div class="filters-tabs" role="tablist">
        <button
          v-for="tab in (['全部', '已发布', '未发布'] as const)"
          :key="tab"
          type="button"
          class="tab-btn"
          :class="{ active: filterTab === tab }"
          @click="filterTab = tab"
        >
          {{ tab }}
        </button>
      </div>

      <input
        v-model="searchQuery"
        type="search"
        placeholder="搜索模块名称或标识…"
        class="search-box"
        aria-label="搜索模块"
      />
    </div>

    <!-- 加载中 -->
    <AdminDataLoading v-if="pending && !response" :pending="pending" :has-data="!!response" />

    <!-- 错误状态 -->
    <div v-else-if="error" class="empty-box">
      <p>加载失败，请检查网络或确认管理员登录状态。</p>
      <button type="button" class="retry-btn" @click="() => refresh()">重试</button>
    </div>

    <!-- 模块表格 -->
    <div v-else-if="filteredModules.length > 0" class="table-wrap">
      <table class="module-table">
        <thead>
          <tr>
            <th>模块名称 / 访问标识</th>
            <th>状态</th>
            <th>章节数量</th>
            <th>内容分类 / 难度</th>
            <th class="text-right">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="m in filteredModules" :key="m.id">
            <td>
              <div class="title-cell">
                <strong class="module-name">
                  {{ m.title }}
                </strong>
                <span v-if="m.source" class="module-slug">{{m.source.path}}</span>
                <span class="module-slug">/learn/modules/{{ m.slug }}</span>
              </div>
            </td>
            <td>
              <div class="status-cell">
                <span v-if="m.status === 'published'" class="badge-status published">已发布</span>
                <span v-else class="badge-status unpublished">未发布</span>
              </div>
            </td>
            <td>{{ m.chapterCount }} 章节</td>
            <td>
              <span>{{ m.category?.name || '未分类' }}</span> · <span class="diff-tag">{{ difficultyLabels[m.difficulty] }}</span>
            </td>
            <td class="text-right">
              <div class="action-cell">
                <NuxtLink :to="`/admin/modules/${m.id}`" class="action-btn edit-btn">
                  查看详情
                </NuxtLink>
                <button v-if="m.status !== 'published'" class="action-btn delete-btn" type="button" :disabled="!!deleting" @click="deleteModule(m.id)"><UiBusyLabel :busy="deleting === m.id">删除</UiBusyLabel></button>
                <NuxtLink
                  v-if="m.status === 'published'"
                  :to="`/learn/modules/${m.slug}`"
                  target="_blank"
                  class="action-btn view-btn"
                  rel="noopener noreferrer"
                >
                  前台预览 <ExternalLink :size="12" aria-hidden="true" />
                </NuxtLink>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 空列表 -->
    <div v-else class="empty-box">
      <p v-if="searchQuery">未找到与“{{ searchQuery }}”匹配的模块。</p>
      <p v-else>暂无模块。在 Git 仓库添加教程后，点击“检查 Git 更新”导入。</p>
    </div>

    <p class="footnote">新导入模块默认未发布；已有模块同步后保留发布状态。</p>

  </div>
</template>

<style scoped>
.module-source-actions{display:flex;align-items:center;gap:14px}.module-source-actions a{text-decoration:none}

.page-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 750;
  margin: 0 0 6px;
  color: var(--ink);
}

.page-desc {
  font-size: 13px;
  color: var(--muted);
  margin: 0;
}

.btn-create {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 8px 18px;
  background: var(--ink);
  color: var(--paper);
  border: 1px solid var(--ink);
  border-radius: 7px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.15s ease;
}

.btn-create:hover {
  opacity: 0.9;
}

/* 筛选条 */
.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
}

.filters-tabs {
  display: flex;
  gap: 6px;
}

.tab-btn {
  padding: 6px 14px;
  border-radius: 6px;
  border: 1px solid var(--line);
  background: var(--paper);
  color: var(--muted);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.tab-btn.active {
  background: var(--green);
  color: var(--on-accent);
  border-color: var(--green);
  font-weight: 600;
}

.search-box {
  width: 260px;
  padding: 7px 12px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--paper);
  color: var(--ink);
  font-size: 13px;
  outline: none;
}

.search-box:focus {
  border-color: var(--green);
}

/* 表格容器 */
.table-wrap {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 10px;
  overflow-x: auto;
}

.module-table {
  width: 100%;
  min-width: 940px;
  border-collapse: collapse;
  text-align: left;
  font-size: 13px;
}

.module-table th {
  padding: 12px 18px;
  border-bottom: 1px solid var(--line);
  background: var(--soft);
  color: var(--muted);
  font-weight: 600;
  font-size: 12px;
}

.module-table th:last-child { width: 230px; }

.module-table td {
  vertical-align: middle;
  padding: 14px 18px;
  border-bottom: 1px solid var(--line);
  color: var(--ink);
}

.module-table tr:last-child td {
  border-bottom: none;
}

.title-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.module-name {
  font-size: 14px;
  color: var(--ink);
}

.module-slug {
  font-family: monospace;
  font-size: 11px;
  color: var(--muted);
  overflow-wrap: anywhere;
}

.status-cell {
  display: flex;
  gap: 6px;
}

.badge-status {
  font-size: 11px;
  padding: 2px 7px;
  border-radius: 4px;
  font-weight: 500;
}

.badge-status.published {
  background: var(--success-bg);
  color: var(--success-ink);
}

.badge-status.unpublished {
  background: var(--soft);
  color: var(--muted);
}

.diff-tag {
  font-family: monospace;
  font-size: 11px;
  color: var(--muted);
}

/* Reserve the same slots in published and draft rows. */
.action-cell {
  display: grid;
  grid-template-columns: 48px 40px 92px;
  justify-content: end;
  align-items: center;
  gap: 8px;
}

.action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  min-height: 32px;
  padding: 0 5px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--muted);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
  text-decoration: none;
  cursor: pointer;
}
.action-btn:hover { background: var(--soft); color: var(--ink); }
.edit-btn { color: var(--green); }
.delete-btn:hover { background: var(--error-bg); color: var(--error-ink); }

.text-right {
  text-align: right;
}

.empty-box {
  padding: 48px;
  text-align: center;
  background: var(--paper);
  border: 1px dashed var(--line);
  border-radius: 10px;
  color: var(--muted);
  font-size: 13px;
}

.retry-btn {
  margin-top: 10px;
  padding: 6px 14px;
  background: var(--ink);
  color: var(--paper);
  border-radius: 6px;
  border: none;
  cursor: pointer;
}

.footnote {
  font-size: 12px;
  color: var(--muted);
  margin-top: 16px;
}

</style>
