<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ArrowLeft, ArrowUp, ArrowDown, X, Plus } from 'lucide-vue-next'
import type { LearningRoute } from '~/shared/routes'

definePageMeta({ layout: 'admin', key: route => route.fullPath })
useHead({ title: '编辑路线 — AttackBox' })

const id = String(useRoute().params.id)
const isNew = id === 'new'

const { data: source } = await useLazyFetch<{ data: { id: string; title: string; status: string }[] }>(
  '/api/admin/modules',
  { key: 'admin:route-module-options' }
)

const title = ref('')
const summary = ref('')
const ids = ref<string[]>([])
const version = ref(0)
const state = ref('unpublished')
const notice = ref('')
const busy = ref(false)
const loaded = ref(isNew)
const query = ref('')

const request = useRequestFetch()
const initialLoading = ref(!isNew)
onMounted(async () => {
 try {
if (!isNew) {
  try {
    const r = await request<{ data: LearningRoute }>(`/api/admin/routes/${id}`)
    title.value = r.data.title
    summary.value = r.data.summary
    ids.value = r.data.modules!.map(m => m.id)
    version.value = r.data.lockVersion
    state.value = r.data.status
    loaded.value = true
  } catch {
    notice.value = '无法读取路线，请返回列表重试'
  }
}
 } finally {initialLoading.value=false}
})

const selected = computed(() =>
  ids.value.map(id => source.value?.data.find(m => m.id === id)).filter(Boolean)
)

const options = computed(() =>
  source.value?.data.filter(
    m => !ids.value.includes(m.id) && m.title.toLowerCase().includes(query.value.toLowerCase())
  ) || []
)

function move(index: number, delta: number) {
  const next = [...ids.value]
  ;[next[index], next[index + delta]] = [next[index + delta]!, next[index]!]
  ids.value = next
}

async function save(intent: 'draft' | 'publish') {
  if (busy.value || !loaded.value) return
  busy.value = true
  notice.value = ''
  try {
    const r = await $fetch<{ data: { id: string } }>(
      isNew ? '/api/admin/routes' : `/api/admin/routes/${id}`,
      {
        method: isNew ? 'POST' : 'PUT',
        body: {
          title: title.value,
          summary: summary.value,
          moduleIds: ids.value,
          lockVersion: version.value,
          intent,
        },
      }
    )
    await clearNuxtData('admin:routes')
    if (isNew) {
      await navigateTo(`/admin/routes/${r.data.id}`)
    } else {
      version.value++
      state.value = intent === 'publish' ? 'published' : 'unpublished'
      notice.value = intent === 'publish' ? '已保存并发布' : '已保存为草稿'
    }
  } catch (e: any) {
    notice.value = e?.data?.message || '保存失败'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="editor">
    <AdminDataLoading :pending="initialLoading" :has-data="loaded" />
    <NuxtLink to="/admin/routes" class="back">
      <ArrowLeft :size="14" aria-hidden="true" />
      <span>路线列表</span>
    </NuxtLink>

    <header class="editor-header">
      <div class="title-group">
        <h1>{{ isNew ? '新建路线' : '编辑路线' }}</h1>
        <UiBadge :variant="state === 'published' ? 'success' : 'neutral'" dot>
          {{ state === 'published' ? '已发布' : '草稿' }}
        </UiBadge>
        <UiBadge v-if="dirty" variant="warning">未保存修改</UiBadge>
      </div>
      <div class="save-actions">
        <UiButton
          type="button"
          variant="secondary"
          size="sm"
          :disabled="busy || !loaded"
          @click="save('draft')"
        >
          保存为草稿
        </UiButton>
        <UiButton
          type="button"
          variant="primary"
          size="sm"
          :loading="busy"
          :disabled="busy || !loaded"
          @click="save('publish')"
        >
          保存并发布
        </UiButton>
      </div>
    </header>

    <UiNotice v-if="notice" variant="info" class="mb-5">{{ notice }}</UiNotice>

    <div v-if="loaded" class="grid">
      <section class="panel">
        <h2>路线信息</h2>
        <label>
          名称 <span>*</span>
          <input v-model="title" maxlength="160" placeholder="例如：Web 安全基础" />
        </label>
        <label>
          简介
          <textarea
            v-model="summary"
            maxlength="500"
            rows="4"
            placeholder="说明这条路线适合谁、涵盖什么。"
          />
        </label>

        <h2 class="outline-title">
          <span>模块顺序</span>
          <small>{{ ids.length }} / 100</small>
        </h2>
        <p v-if="!ids.length" class="muted">从右侧添加模块，再按学习顺序排列。</p>

        <ol>
          <li v-for="(item, index) in selected" :key="item!.id">
            <span class="number">{{ String(index + 1).padStart(2, '0') }}</span>
            <div class="module-name">
              {{ item!.title }}
              <small v-if="item!.status !== 'published'">尚未发布</small>
            </div>
            <button
              type="button"
              :aria-label="`上移 ${item!.title}`"
              :disabled="index === 0"
              @click="move(index, -1)"
            >
              <ArrowUp :size="15" aria-hidden="true" />
            </button>
            <button
              type="button"
              :aria-label="`下移 ${item!.title}`"
              :disabled="index === ids.length - 1"
              @click="move(index, 1)"
            >
              <ArrowDown :size="15" aria-hidden="true" />
            </button>
            <button
              type="button"
              :aria-label="`移除 ${item!.title}`"
              @click="ids.splice(index, 1)"
            >
              <X :size="15" aria-hidden="true" />
            </button>
          </li>
        </ol>
      </section>

      <aside class="panel">
        <h2>添加模块</h2>
        <input v-model="query" aria-label="搜索模块" placeholder="搜索模块名称" />
        <div class="choices">
          <button
            v-for="item in options"
            :key="item.id"
            type="button"
            :disabled="ids.length >= 100"
            @click="ids.push(item.id)"
          >
            <div>
              {{ item.title }}
              <small>
                {{ item.status === 'published' ? '已发布' : '草稿，发布路线前需先发布模块' }}
              </small>
            </div>
            <Plus :size="16" aria-hidden="true" />
          </button>
          <p v-if="!options.length" class="muted">没有可添加的模块</p>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.editor {
  width: 100%;
}

.editor-header,
.title-group,
.save-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.editor-header {
  justify-content: space-between;
  flex-wrap: wrap;
  margin: 20px 0 24px;
}

h1 {
  font-size: 24px;
  font-weight: 750;
  margin: 0;
}

.status-badge {
  font-size: 11px;
  padding: 3px 8px;
  border-radius: 5px;
  background: var(--soft);
  color: var(--muted);
  font-weight: 550;
}

.status-badge.published {
  background: var(--success-bg);
  color: var(--success-ink);
}

.back {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  color: var(--muted);
  font-size: 12px;
  text-decoration: none;
}

.muted,
small {
  color: var(--muted);
  font-size: 12px;
}

.grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 24px;
  align-items: start;
}

.panel {
  padding: 24px;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 10px;
}

h2 {
  font-size: 15px;
  font-weight: 650;
  margin: 0 0 20px;
}

label {
  display: block;
  font-size: 13px;
  font-weight: 550;
  margin-bottom: 22px;
}

label span {
  color: var(--error-ink);
}

input:not([type='checkbox']),
textarea {
  display: block;
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--line);
  background: var(--bg);
  color: var(--ink);
  border-radius: 6px;
  padding: 10px 12px;
  margin-top: 8px;
  font: inherit;
  font-size: 13px;
}

button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--paper);
  color: var(--ink);
  padding: 8px 14px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 550;
}

button:disabled {
  opacity: 0.4;
  cursor: default;
}

.primary {
  background: var(--ink);
  color: var(--paper);
  border-color: var(--ink);
}

.outline-title {
  padding-top: 24px;
  border-top: 1px solid var(--line);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

ol {
  padding: 0;
  margin: 0;
  list-style: none;
}

li {
  display: flex;
  gap: 9px;
  align-items: center;
  border-top: 1px solid var(--line);
  padding: 16px 0;
  font-size: 13px;
}

.number {
  color: var(--muted);
  width: 30px;
  font-family: ui-monospace, monospace;
}

.module-name {
  flex: 1;
  min-width: 0;
}

small {
  display: block;
  margin-top: 5px;
}

li button {
  padding: 6px;
}

.choices {
  max-height: 600px;
  overflow: auto;
  margin-top: 16px;
}

.choices button {
  width: 100%;
  text-align: left;
  justify-content: space-between;
  border: 0;
  border-bottom: 1px solid var(--line);
  border-radius: 0;
  padding: 14px 4px;
}

.choices button:hover:not(:disabled) {
  background: var(--soft);
}

.notice {
  background: var(--soft);
  padding: 14px 16px;
  font-size: 13px;
  border-radius: 6px;
  margin-bottom: 20px;
}

</style>
