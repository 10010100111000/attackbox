<script setup lang="ts">
import { ref, computed } from 'vue'
import { Plus, Search, RefreshCw, Box } from 'lucide-vue-next'
import type { TargetTemplateList } from '~/shared/target-templates'

definePageMeta({ layout: 'admin' })
useHead({ title: '靶场模板 — AttackBox' })

const input = ref('')
const search = ref('')
const page = ref(1)

const { data, pending, error, refresh } = await useLazyFetch<{ data: TargetTemplateList }>(
  '/api/admin/target-templates',
  {
    key: 'admin:target-templates',
    query: computed(() => ({ search: search.value, page: page.value })),
  }
)

function submit() {
  page.value = 1
  search.value = input.value.trim()
}
</script>

<template>
  <div class="users-page">
    <header class="page-heading">
      <div>
        <h1>靶场模板</h1>
        <p>集中维护教学与挑战环境配置。启用表示允许关联，不代表容器正在运行。</p>
      </div>
      <NuxtLink to="/admin/target-templates/new">
        <Plus :size="16" aria-hidden="true" />
        <span>新建模板</span>
      </NuxtLink>
    </header>
    <AdminDataLoading :pending="pending" :has-data="!!data" />

    <form class="filters" @submit.prevent="submit">
      <label class="search">
        <Search :size="15" aria-hidden="true" />
        <input v-model="input" aria-label="搜索模板" placeholder="搜索模板名称" maxlength="100" />
      </label>
      <button type="submit">搜索</button>
      <button type="button" :disabled="pending" @click="refresh()">
        <RefreshCw :size="14" aria-hidden="true" />
        <span>刷新</span>
      </button>
      <span class="count muted">{{ data?.data.total ?? '—' }} 个模板</span>
    </form>

    <p v-if="error" class="notice" role="alert">
      无法读取模板，请检查登录状态或重试。
    </p>

    <div v-else class="panel table-wrap" :aria-busy="pending">
      <table>
        <thead>
          <tr>
            <th>模板</th>
            <th>部署文件</th>
            <th>服务端口</th>
            <th>允许关联</th>
            <th class="actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in data?.data.items" :key="item.id">
            <td class="template-cell">
              <NuxtLink :to="`/admin/target-templates/${item.id}`" class="user-name">
                {{ item.name }}
              </NuxtLink>
              <span class="secondary">{{ item.description }}</span>
            </td>
            <td>
              <code class="image-code">{{ item.settings.file }}</code>
            </td>
            <td>
              {{ item.settings.entryPort }}
            </td>
            <td>
              <span class="status" :class="{ active: item.enabled }">
                {{ item.enabled ? '已启用' : '已停用' }}
              </span>
            </td>
            <td class="actions">
              <NuxtLink :to="`/admin/target-templates/${item.id}`">编辑</NuxtLink>
            </td>
          </tr>
        </tbody>
      </table>

      <div v-if="!pending && !data?.data.items.length" class="empty">
        <Box :size="26" aria-hidden="true" />
        <p>{{ pending ? '加载中…' : search ? '没有符合条件的模板' : '还没有靶场模板' }}</p>
      </div>
    </div>

    <footer v-if="data" class="pagination">
      <span class="muted">第 {{ page }} 页 · 每页 20 个</span>
      <div>
        <button type="button" :disabled="pending || page === 1" @click="page--">上一页</button>
        <button
          type="button"
          :disabled="pending || page * 20 >= data.data.total"
          @click="page++"
        >
          下一页
        </button>
      </div>
    </footer>
  </div>
</template>

<style scoped src="~/assets/css/admin-users.css"></style>
