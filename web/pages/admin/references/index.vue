<script setup lang="ts">
import { BookOpen, GitBranch, Settings, Download, Eye } from 'lucide-vue-next'
import type {
  AdminReferenceSummary,
  ReferencePage,
  ReferenceScan,
  ReferenceImportResult
} from '~/shared/references'
import { apiErrorMessage, apiFetch } from '~/utils/api'
import { useSequentialBatch } from '~/composables/useSequentialBatch'
import { useEditorGuard } from '~/composables/useEditorGuard'
definePageMeta({ layout: 'admin' })
useHead({ title: '参考资料管理 — AttackBox' })
const route = useRoute(),
  page = computed(() => Number(route.query.page || 1))
const { data, pending, error, refresh } = await useLazyFetch<{
  data: ReferencePage<AdminReferenceSummary>
}>('/api/admin/references', { query: { page }, key: 'admin:references' })
const settingsOpen = ref(false),
  importing = ref(false),
  scanning = ref(false),
  scan = ref<ReferenceScan | null>(null),
  notice = ref('')
const settingsPanel = ref<{ canLeave: () => Promise<boolean> } | null>(null)
const operation = ref('导入')
const importSelection = ref(new Set<string>())
const publishSelection = ref(new Map<string, { id: string; title: string; lockVersion: number }>())
const { running, processed, total, active, results, succeeded, stopped, run, reset, stop } =
  useSequentialBatch()
const busy = computed(() => running.value || scanning.value)
const candidates = computed(
  () => scan.value?.items.filter(item => item.state === 'new' || item.state === 'changed') || []
)
const selectedImports = computed(() =>
  candidates.value.filter(item => importSelection.value.has(item.sourcePath))
)
const unpublished = computed(
  () => data.value?.data.items.filter(item => item.status === 'unpublished') || []
)
const allImports = computed(
  () => candidates.value.length > 0 && selectedImports.value.length === candidates.value.length
)
const allPublish = computed(
  () =>
    unpublished.value.length > 0 &&
    unpublished.value.every(item => publishSelection.value.has(item.id))
)
const confirmAction = useConfirm()
useEditorGuard({ dirty: false, busy, title: '资料处理中', description: '请等待当前操作完成。' })
watch(data, response => {
  if (running.value || !response) return
  for (const item of response.data.items)
    if (publishSelection.value.has(item.id)) {
      if (item.status === 'published') publishSelection.value.delete(item.id)
      else
        publishSelection.value.set(item.id, {
          id: item.id,
          title: item.title,
          lockVersion: item.lockVersion
        })
    }
})
const stateLabels = {
  new: '待导入',
  changed: '有更新',
  unchanged: '已同步',
  missing: '源文件已移除'
}
async function toggleSettings() {
  if (busy.value) return
  if (settingsOpen.value && settingsPanel.value && !(await settingsPanel.value.canLeave())) return
  settingsOpen.value = !settingsOpen.value
}
async function check() {
  if (busy.value) return
  reset()
  importing.value = true
  scanning.value = true
  notice.value = ''
  try {
    scan.value = (
      await apiFetch<{ data: ReferenceScan }>('/api/admin/references/scan', {
        method: 'POST',
        retry: false
      })
    ).data
    importSelection.value = new Set(candidates.value.map(item => item.sourcePath))
  } catch (e) {
    notice.value = apiErrorMessage(e, '扫描失败，请检查仓库设置')
    settingsOpen.value = true
  } finally {
    scanning.value = false
  }
}
function selectImport(path: string, checked: boolean) {
  if (checked) importSelection.value.add(path)
  else importSelection.value.delete(path)
}
function selectPublish(item: { id: string; title: string; lockVersion: number }, checked: boolean) {
  if (checked) publishSelection.value.set(item.id, item)
  else publishSelection.value.delete(item.id)
}
async function load(items: ReferenceScan['items']) {
  if (busy.value || !scan.value || !items.length) return
  const updates = items.filter(item => item.state === 'changed').length
  if (
    updates &&
    !(await confirmAction({
      title: `导入 ${items.length} 篇资料？`,
      description: `${updates} 篇更新将替换本地内容并转为未发布。导入后可单独预览或批量发布。`,
      confirmLabel: '导入为未发布',
      tone: 'neutral'
    }))
  )
    return
  const snapshot = { commit: scan.value.commit, version: scan.value.version }
  notice.value = ''
  operation.value = '导入'
  await run(
    items,
    item => `source:${item.sourcePath}`,
    async item => {
      const { data: imported } = await apiFetch<{ data: ReferenceImportResult }>(
        '/api/admin/references/import',
        {
          method: 'POST',
          body: { ...snapshot, sourcePath: item.sourcePath, lockVersion: item.lockVersion },
          retry: false
        }
      )
      const local = data.value?.data.items.find(row => row.id === imported.id)
      if (local) {
        local.status = imported.status
        local.lockVersion = imported.lockVersion
      }
      item.id = imported.id
      item.lockVersion = imported.lockVersion
      item.state = 'unchanged'
      importSelection.value.delete(item.sourcePath)
      if (imported.status === 'unpublished')
        selectPublish(
          { id: imported.id, title: item.title, lockVersion: imported.lockVersion },
          true
        )
    }
  )
  await refresh()
}
async function publish() {
  if (busy.value || !publishSelection.value.size) return
  const items = [...publishSelection.value.values()]
  if (
    !(await confirmAction({
      title: `发布 ${items.length} 篇资料？`,
      description: '发布后，所选资料的正文与图片将公开访问。',
      confirmLabel: '发布所选资料',
      tone: 'neutral'
    }))
  )
    return
  notice.value = ''
  operation.value = '发布'
  await run(
    items,
    item => `id:${item.id}`,
    async item => {
      await apiFetch(`/api/admin/references/${item.id}`, {
        method: 'PUT',
        body: { intent: 'publish', lockVersion: item.lockVersion },
        retry: false
      })
      publishSelection.value.delete(item.id)
      const local = data.value?.data.items.find(row => row.id === item.id)
      if (local) {
        local.status = 'published'
        local.lockVersion++
      }
    }
  )
  clearNuxtData(key => key.startsWith('references:'))
  await refresh()
}
function configured() {
  scan.value = null
  importSelection.value.clear()
  settingsOpen.value = false
}
</script>
<template>
  <div class="reference-admin">
    <header class="page-heading">
      <div>
        <h1>参考资料</h1>
        <p>从 Git 批量导入，按需预览，再统一发布。</p>
      </div>
      <div class="actions">
        <UiButton
          variant="secondary"
          size="sm"
          as="a"
          href="/api/admin/references/template"
          download="参考资料示例.md"
          >下载 MD 示例</UiButton
        >
        <UiButton
          variant="secondary"
          size="sm"
          :aria-expanded="settingsOpen"
          :disabled="busy"
          @click="toggleSettings"
          ><Settings :size="14" aria-hidden="true" />仓库设置</UiButton
        >
        <UiButton variant="primary" size="sm" :loading="scanning" :disabled="busy" @click="check"
          ><GitBranch :size="14" aria-hidden="true" />扫描仓库</UiButton
        >
      </div>
    </header>
    <section v-if="total" class="batch-progress panel" aria-label="批量处理进度">
      <div>
        <strong>{{ operation }}{{ running ? '中' : stopped ? '已停止' : '完成' }}</strong
        ><span
          >{{ processed }} / {{ total }} · 成功 {{ succeeded }} · 失败
          {{ processed - succeeded }}</span
        >
      </div>
      <UiButton v-if="running" variant="secondary" size="xs" :disabled="stopped" @click="stop">{{
        stopped ? '完成当前项后停止' : '停止后续操作'
      }}</UiButton>
    </section>
    <UiNotice v-if="notice" variant="error" class="mb-5">{{ notice }}</UiNotice>
    <AdminRepositorySettings
      v-if="settingsOpen"
      ref="settingsPanel"
      endpoint="/api/admin/references/repository"
      @saved="configured"
    />
    <section v-if="importing" class="panel" aria-label="Git 资料导入">
      <div class="page-heading">
        <h2>仓库资料</h2>
        <div class="actions">
          <UiButton
            size="sm"
            :disabled="busy || !selectedImports.length"
            @click="load(selectedImports)"
            ><Download :size="14" aria-hidden="true" />{{
              allImports ? '导入全部变更' : '导入所选'
            }}（{{ selectedImports.length }}）</UiButton
          >
          <UiButton variant="secondary" size="xs" :disabled="busy" @click="importing = false"
            >收起</UiButton
          >
        </div>
      </div>
      <UiBusyIndicator v-if="scanning" label="扫描资料仓库" />
      <template v-else-if="scan"
        ><p class="muted">
          扫描版本 {{ scan.commit.slice(0, 12) }} · {{ candidates.length }} 项待导入
        </p>
        <div v-if="scan.items.length" class="repository-results">
          <table>
            <thead>
              <tr>
                <th class="selection">
                  <UiCheckbox
                    aria-label="选择全部变更"
                    :model-value="allImports"
                    :disabled="busy || !candidates.length"
                    @update:model-value="
                      checked =>
                        (importSelection = new Set(
                          checked ? candidates.map(item => item.sourcePath) : []
                        ))
                    "
                  />
                </th>
                <th>文件</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in scan.items" :key="item.sourcePath">
                <td class="selection">
                  <UiCheckbox
                    :aria-label="`选择导入 ${item.title}`"
                    :model-value="importSelection.has(item.sourcePath)"
                    :disabled="busy || (item.state !== 'new' && item.state !== 'changed')"
                    @update:model-value="checked => selectImport(item.sourcePath, checked)"
                  />
                </td>
                <td class="reference-name">
                  {{ item.title }}<small>{{ item.sourcePath }}</small>
                </td>
                <td>{{ stateLabels[item.state] }}</td>
                <td>
                  <div class="row-actions">
                    <UiButton
                      v-if="item.state === 'new' || item.state === 'changed'"
                      variant="secondary"
                      size="xs"
                      :loading="active === `source:${item.sourcePath}`"
                      :disabled="busy"
                      @click="load([item])"
                      >{{ item.state === 'new' ? '导入' : '导入更新' }}</UiButton
                    >
                    <UiButton
                      v-if="item.id"
                      :to="`/admin/reference-preview/${item.id}`"
                      target="_blank"
                      variant="secondary"
                      size="xs"
                      ><Eye :size="13" aria-hidden="true" />预览</UiButton
                    >
                    <span v-if="item.state === 'missing'" class="muted">本地资料保留</span>
                  </div>
                  <small
                    v-if="results[`source:${item.sourcePath}`]?.ok === false"
                    class="operation-error"
                    role="alert"
                    >{{ results[`source:${item.sourcePath}`]?.message }}</small
                  >
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty">目录中没有资料 Markdown 文件。</div>
      </template>
    </section>
    <AdminDataLoading :pending="pending" :has-data="!!data" />
    <UiNotice v-if="error" variant="error" class="mb-5">资料列表读取失败。</UiNotice>
    <section v-if="data?.data.items.length" class="panel">
      <div class="page-heading">
        <h2>本地资料</h2>
        <div class="actions">
          <span class="muted">已选择 {{ publishSelection.size }} 篇待发布</span
          ><UiButton
            v-if="publishSelection.size"
            variant="secondary"
            size="xs"
            :disabled="busy"
            @click="publishSelection.clear()"
            >清空选择</UiButton
          ><UiButton size="sm" :disabled="busy || !publishSelection.size" @click="publish"
            >发布所选（{{ publishSelection.size }}）</UiButton
          >
        </div>
      </div>
      <table>
        <thead>
          <tr>
            <th class="selection">
              <UiCheckbox
                aria-label="选择本页未发布资料"
                :model-value="allPublish"
                :disabled="busy || !unpublished.length"
                @update:model-value="
                  checked => unpublished.forEach(item => selectPublish(item, checked))
                "
              />
            </th>
            <th>资料</th>
            <th>状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in data.data.items" :key="item.id">
            <td class="selection">
              <UiCheckbox
                :aria-label="`选择发布 ${item.title}`"
                :model-value="publishSelection.has(item.id)"
                :disabled="busy || item.status === 'published'"
                @update:model-value="checked => selectPublish(item, checked)"
              />
            </td>
            <td class="reference-name">
              <NuxtLink class="reference-link" :to="`/admin/references/${item.id}`">{{
                item.title
              }}</NuxtLink
              ><small>{{ item.sourcePath }}</small>
            </td>
            <td>
              <UiBadge :variant="item.status === 'published' ? 'success' : 'neutral'" dot>{{
                item.status === 'published' ? '已发布' : '未发布'
              }}</UiBadge>
            </td>
            <td>
              <div class="row-actions">
                <UiButton
                  variant="secondary"
                  size="xs"
                  :to="`/admin/reference-preview/${item.id}`"
                  target="_blank"
                  >预览</UiButton
                ><UiButton
                  variant="secondary"
                  size="xs"
                  :disabled="busy"
                  :to="`/admin/references/${item.id}`"
                  >管理</UiButton
                >
              </div>
              <small
                v-if="results[`id:${item.id}`]?.ok === false"
                class="operation-error"
                role="alert"
                >{{ results[`id:${item.id}`]?.message }}</small
              >
            </td>
          </tr>
        </tbody>
      </table>
    </section>
    <UiEmptyState
      v-else-if="!pending && !error"
      class="panel"
      description="从 Git 导入第一篇参考资料。"
      ><template #icon><BookOpen :size="30" :stroke-width="1.3" aria-hidden="true" /></template
    ></UiEmptyState>
    <nav v-if="data && data.data.total > 24" class="pages" aria-label="资料管理分页">
      <UiButton
        v-if="page > 1"
        variant="secondary"
        size="xs"
        :disabled="busy"
        :to="{ query: { page: page - 1 } }"
        >上一页</UiButton
      ><span>{{ page }}</span
      ><UiButton
        v-if="page * 24 < data.data.total"
        variant="secondary"
        size="xs"
        :disabled="busy"
        :to="{ query: { page: page + 1 } }"
        >下一页</UiButton
      >
    </nav>
  </div>
</template>

<style scoped src="~/assets/css/reference-admin.css"></style>
