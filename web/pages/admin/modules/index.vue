<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useFetch, useHead } from '#imports'
import type { AdminModuleListItem } from '~/shared/content'

definePageMeta({
  layout: 'admin'
})

useHead({
  title: '学习模块管理 — AttackBox'
})

type AdminModuleItem = AdminModuleListItem

const router = useRouter()
const { data: response, pending, error, refresh } = await useFetch<{ ok: boolean; data: AdminModuleItem[] }>(
  '/api/admin/modules'
)

const modules = computed<AdminModuleItem[]>(() => {
  return response.value?.data || []
})

const filterTab = ref<'全部' | '已发布' | '草稿'>('全部')
const searchQuery = ref('')

const filteredModules = computed(() => {
  let list = modules.value

  if (filterTab.value === '已发布') {
    list = list.filter(m => m.isPublished)
  } else if (filterTab.value === '草稿') {
    list = list.filter(m => !m.isPublished || m.hasDraft)
  }

  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(m => {
      const title = (m.draft?.title || m.published?.title || '').toLowerCase()
      const slug = m.slug.toLowerCase()
      return title.includes(q) || slug.includes(q)
    })
  }

  return list
})

async function deleteModule(id: string) {
  if (!confirm('删除该模块？模块将从管理列表和学习页移除，历史内容保留。')) return
  try { await $fetch(`/api/admin/modules/${id}`, { method: 'DELETE' }); await refresh() }
  catch (error: any) { alert(error?.data?.message || '删除失败') }
}
</script>

<template>
  <div class="admin-modules-page">
    <div class="page-head">
      <div>
        <h1 class="page-title">学习模块</h1>
        <p class="page-desc">管理章节、Markdown 来源与章节练习。</p>
      </div>
      <button type="button" class="btn-create" @click="router.push('/admin/modules/new')">
        ＋ 新建模块
      </button>
    </div>

    <!-- 筛选与搜索工具条 -->
    <div class="filter-bar">
      <div class="filters-tabs" role="tablist">
        <button
          v-for="tab in (['全部', '已发布', '草稿'] as const)"
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
    <div v-if="pending" class="empty-box">
      正在加载模块清单…
    </div>

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
                  {{ m.draft?.title || m.published?.title || '未命名模块' }}
                </strong>
                <span class="module-slug">/learn/{{ m.slug }}</span>
              </div>
            </td>
            <td>
              <div class="status-cell">
                <span v-if="m.isPublished" class="badge-status published">已发布</span>
                <span v-else class="badge-status draft">草稿</span>
                <span v-if="m.isPublished && m.hasDraft" class="badge-status modified">有新草稿</span>
              </div>
            </td>
            <td>{{ m.chapterCount }} 章节</td>
            <td>
              <span>{{ m.category?.name || '未分类' }}</span> · <span class="diff-tag">{{ m.draft?.difficulty || m.published?.difficulty || 'beginner' }}</span>
            </td>
            <td class="text-right">
              <div class="action-cell">
                <NuxtLink :to="`/admin/modules/${m.id}`" class="action-btn edit-btn">
                  编辑内容
                </NuxtLink>
                <button class="action-btn" type="button" @click="deleteModule(m.id)">删除</button>
                <NuxtLink
                  v-if="m.isPublished"
                  :to="`/learn/modules/${m.slug}`"
                  target="_blank"
                  class="action-btn view-btn"
                >
                  前台预览 ↗
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
      <p v-else>暂无任何模块，点击上方按钮新建第一个模块。</p>
    </div>

    <p class="footnote">已发布模块的修改先保留为草稿，重新发布后才替换线上版本。</p>

  </div>
</template>

<style scoped>
.admin-modules-page {
  max-width: 1100px;
}

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
  overflow: hidden;
}

.module-table {
  width: 100%;
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

.module-table td {
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

.badge-status.draft {
  background: var(--soft);
  color: var(--muted);
}

.badge-status.modified {
  background: rgba(255, 175, 136, 0.15);
  color: var(--accent);
}

.diff-tag {
  font-family: monospace;
  font-size: 11px;
  color: var(--muted);
}

.action-cell {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.action-btn {
  font-size: 12px;
  font-weight: 600;
  text-decoration: none;
}

.edit-btn {
  color: var(--green);
}

.edit-btn:hover {
  text-decoration: underline;
}

.view-btn {
  color: var(--muted);
}

.view-btn:hover {
  color: var(--ink);
}

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
