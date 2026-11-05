<script setup lang="ts">
import { ref, computed, onMounted, watch, onBeforeUnmount } from 'vue'
import { useEditorGuard } from '~/composables/useEditorGuard'
import { ArrowLeft, Check, Zap } from 'lucide-vue-next'
import TargetSourcePanel from '~/components/admin/TargetSourcePanel.vue'
import TargetNodesPanel from '~/components/admin/TargetNodesPanel.vue'
import type { TargetTemplate } from '~/shared/target-templates'
import type { ComposePreview, ComposeSettings, ComposePreparation } from '~/shared/compose-labs'
import { apiErrorMessage, apiData } from '~/utils/api'

const confirmAction = useConfirm()
definePageMeta({ layout: 'admin', key: r => r.fullPath })
useHead({ title: 'Compose 靶场模板 — AttackBox' })

const route = useRoute(),
  id = String(route.params.id),
  isNew = id === 'new',
  request = useRequestFetch()

const busy = ref(false),
  loaded = ref(isNew),
  message = ref(''),
  failed = ref(false),
  preview = ref<ComposePreview | null>(null),
  bundleId = ref(''),
  version = ref(0),
  storedKeys = ref<string[]>([]),
  variables = ref<Record<string, string>>(Object.create(null)),
  clearVariables = ref<string[]>([]),
  checkedCommit = ref('')

const form = ref({ name: '', description: '', accessInstructions: '', enabled: false })
const settings = ref<ComposeSettings>({
  file: 'compose.yml',
  entryService: 'web',
  entryNetwork: 'default',
  entryPort: 8080,
  prefixBits: 28
})

const nodes = ref<
    { id: string; label: string; available: boolean; state: string; bootId: string | null }[]
  >([]),
  preparations = ref<ComposePreparation[]>([]),
  nodeBusy = ref('')

const initial = ref(''),
  configuration = ref('')
let alive = true

const contentKey = () => JSON.stringify({ hash: preview.value?.hash, settings: settings.value })
const stateKey = () =>
  JSON.stringify({
    form: form.value,
    bundle: bundleId.value,
    content: contentKey(),
    variables: variables.value,
    clear: clearVariables.value
  })

const dirty = computed(() => loaded.value && stateKey() !== initial.value)
const configChanged = computed(
  () =>
    contentKey() !== configuration.value ||
    Object.values(variables.value).some(v => v !== '') ||
    clearVariables.value.length > 0
)
const ready = computed(() => !configChanged.value && preparations.value.some(p => !!p.deploymentId))
const issues = computed(() => [
  ...(preview.value?.issues || []),
  ...(preview.value?.missingFiles || []).map(f => `缺少配套文件／目录：${f}`)
])

const labels: Record<string, string> = {
  ready: '已准备',
  preparing: '准备中',
  failed: '准备失败',
  unknown: '待确认',
  cancelling: '取消中',
  cancelled: '已取消'
}

watch(configChanged, value => {
  if (value) form.value.enabled = false
})

function baseline() {
  variables.value = Object.create(null)
  clearVariables.value = []
  configuration.value = contentKey()
  initial.value = stateKey()
}

function showError(e: unknown, fallback: string) {
  message.value = apiErrorMessage(e, fallback)
  failed.value = true
}

useEditorGuard({
  dirty,
  busy: () => loaded.value && busy.value,
  title: '放弃靶场配置？',
  description: '当前未保存的配置将丢失。'
})

onBeforeUnmount(() => {
  alive = false
})

async function loadPreparation() {
  const { data } = await request<{
    data: { items: ComposePreparation[]; nodes: typeof nodes.value }
  }>(`/api/admin/target-templates/${id}/preparations`)
  if (alive) {
    preparations.value = data.items
    nodes.value = data.nodes
  }
}

async function load() {
  busy.value = true
  try {
    const { data: t } = await request<{ data: TargetTemplate }>(`/api/admin/target-templates/${id}`)
    form.value = {
      name: t.name,
      description: t.description,
      accessInstructions: t.accessInstructions,
      enabled: t.enabled
    }
    bundleId.value = t.bundleId!
    settings.value = { ...t.settings! }
    version.value = t.lockVersion
    storedKeys.value = t.variableKeys || []
    preview.value = (
      await request<{ data: ComposePreview }>(`/api/admin/lab-bundles/${bundleId.value}`, {
        query: { file: settings.value.file }
      })
    ).data
    loaded.value = true
    baseline()
    await loadPreparation()
  } catch (e) {
    showError(e, '模板读取失败')
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  if (!isNew) void load()
  else baseline()
})

async function upload(e: Event) {
  const input = e.target as HTMLInputElement,
    file = input.files?.[0]
  if (!file) return
  busy.value = true
  failed.value = false
  message.value = ''
  try {
    const data = await apiData<ComposePreview>('/api/admin/lab-bundles', {
      method: 'POST',
      headers: {
        'content-type': 'application/octet-stream',
        'x-lab-filename': encodeURIComponent(file.name)
      },
      body: file,
      retry: false
    })
    preview.value = data
    bundleId.value = data.id
    settings.value.file = data.composeFiles[0]!
    const defaultSvc =
      data.services.find(s => s.name === 'web') ||
      data.services.find(s => s.ports?.length) ||
      data.services[0]
    settings.value.entryService = defaultSvc?.name || ''
    settings.value.entryNetwork = defaultSvc?.networks[0] || 'default'
    const ports = defaultSvc?.ports
    if (ports?.length)
      settings.value.entryPort = (ports.find(p => p.protocol === 'tcp') || ports[0])!.port
    form.value.enabled = false
    variables.value = Object.create(null)
    clearVariables.value = []
    if (!form.value.name) form.value.name = file.name.replace(/\.(zip|ya?ml)$/i, '')
    message.value = '部署包已读取，请确认入口和配套文件。'
  } catch (err) {
    showError(err, '部署包上传失败')
  } finally {
    busy.value = false
    input.value = ''
  }
}

function applyGitPreview(data: ComposePreview) {
  const previousHash = preview.value?.hash
  preview.value = data
  bundleId.value = data.id
  checkedCommit.value = ''
  if (!data.composeFiles.includes(settings.value.file)) settings.value.file = data.composeFiles[0]!
  const service =
    data.services.find(s => s.name === settings.value.entryService) ||
    data.services.find(s => s.name === 'web') ||
    data.services.find(s => s.ports?.length) ||
    data.services[0]
  settings.value.entryService = service?.name || ''
  if (!service?.networks.includes(settings.value.entryNetwork))
    settings.value.entryNetwork = service?.networks[0] || 'default'
  if (!previousHash) {
    settings.value.entryPort =
      (service?.ports?.find(p => p.protocol === 'tcp') || service?.ports?.[0])?.port || 8080
  }
  if (previousHash !== data.hash) form.value.enabled = false
  variables.value = Object.create(null)
  clearVariables.value = []
  if (!form.value.name)
    form.value.name =
      data.source?.directory.split('/').at(-1) || data.source?.repository.split('/').at(-1) || ''
}

async function importGit(gitPayload: {
  repository: string
  branch: string
  directory: string
  token: string
}) {
  busy.value = true
  failed.value = false
  message.value = ''
  try {
    const data = await apiData<{ bundle: ComposePreview }>('/api/admin/lab-bundles/git', {
      method: 'POST',
      body: gitPayload,
      retry: false
    })
    applyGitPreview(data.bundle)
    message.value = '已导入，请确认部署入口并保存模板。'
  } catch (e) {
    showError(e, 'Git 导入失败')
  } finally {
    busy.value = false
  }
}

async function checkGit() {
  busy.value = true
  failed.value = false
  message.value = ''
  try {
    const data = await apiData<{ latestCommit: string; changed: boolean }>(
      `/api/admin/lab-bundles/${bundleId.value}/git`,
      { method: 'POST', body: { action: 'check' }, retry: false }
    )
    checkedCommit.value = data.changed ? data.latestCommit : ''
    message.value = data.changed ? '发现新提交，确认后可导入更新。' : '当前已是最新提交。'
  } catch (e) {
    showError(e, '检查更新失败')
  } finally {
    busy.value = false
  }
}

async function importUpdate() {
  busy.value = true
  failed.value = false
  try {
    const data = await apiData<{ bundle: ComposePreview; unchanged: boolean }>(
      `/api/admin/lab-bundles/${bundleId.value}/git`,
      {
        method: 'POST',
        body: { action: 'import', expectedCommit: checkedCommit.value, file: settings.value.file },
        retry: false
      }
    )
    applyGitPreview(data.bundle)
    message.value = data.unchanged
      ? '提交已更新，文件内容相同，无需重新准备；保存后记录新来源。'
      : '更新已导入编辑区，请确认并保存；新部署需要重新准备。'
  } catch (e) {
    checkedCommit.value = ''
    showError(e, '导入更新失败')
  } finally {
    busy.value = false
  }
}

async function selectFile(file: string) {
  if (busy.value || !bundleId.value || file === settings.value.file) return
  busy.value = true
  failed.value = false
  message.value = ''
  try {
    const data = await apiData<ComposePreview>(`/api/admin/lab-bundles/${bundleId.value}`, {
      query: { file }
    })
    const service =
      data.services.find(s => s.name === 'web') ||
      data.services.find(s => s.ports?.length) ||
      data.services[0]
    preview.value = data
    settings.value = {
      ...settings.value,
      file,
      entryService: service?.name || '',
      entryNetwork: service?.networks[0] || 'default',
      entryPort:
        (service?.ports?.find(p => p.protocol === 'tcp') || service?.ports?.[0])?.port || 8080
    }
    variables.value = Object.create(null)
    clearVariables.value = []
  } catch (e) {
    showError(e, 'Compose 文件解析失败，原入口配置已保留')
  } finally {
    busy.value = false
  }
}

async function save() {
  if (busy.value || !preview.value || issues.value.length) return
  busy.value = true
  message.value = ''
  failed.value = false
  try {
    const variablesInput = Object.fromEntries(
      Object.entries(variables.value).filter(([, v]) => v !== '')
    )
    const data = await apiData<TargetTemplate>(
      isNew ? '/api/admin/target-templates' : `/api/admin/target-templates/${id}`,
      {
        method: isNew ? 'POST' : 'PUT',
        body: {
          ...form.value,
          bundleId: bundleId.value,
          settings: settings.value,
          variables: variablesInput,
          clearVariables: clearVariables.value,
          lockVersion: version.value
        },
        retry: false
      }
    )
    version.value = data.lockVersion
    storedKeys.value = data.variableKeys || []
    baseline()
    clearNuxtData('admin:target-templates')
    if (isNew) {
      busy.value = false
      await navigateTo(`/admin/target-templates/${data.id}`)
    } else {
      await loadPreparation()
      message.value = '模板已保存'
    }
  } catch (e) {
    showError(e, '保存失败，输入已保留')
  } finally {
    busy.value = false
  }
}

const preparation = (node: string) => preparations.value.find(p => p.nodeId === node)

async function prepare(nodeId: string, action: 'prepare' | 'status' | 'cancel' | 'rebuild') {
  if (nodeBusy.value || (dirty.value && ['prepare', 'rebuild'].includes(action))) return
  if (
    action === 'rebuild' &&
    !(await confirmAction({
      title: '强制重建靶场？',
      description: '重新拉取镜像并忽略构建缓存。现有部署和运行实例会保留。',
      confirmLabel: '强制重建',
      tone: 'neutral'
    }))
  )
    return
  nodeBusy.value = nodeId
  failed.value = false
  try {
    const data = await apiData<ComposePreparation | null>(
      `/api/admin/target-templates/${id}/preparations`,
      {
        method: 'POST',
        body: { nodeId, action, expectedAttemptId: preparation(nodeId)?.attemptId },
        retry: false
      }
    )
    if (data) {
      preparations.value = preparations.value.filter(p => p.nodeId !== nodeId).concat(data)
      message.value =
        data.state === 'preparing'
          ? '准备任务已提交，可手动查询结果。'
          : data.state === 'cancelling'
            ? '已请求取消，请手动查询确认。'
            : data.error || labels[data.state] || '状态已更新'
      failed.value = data.state === 'failed' || data.state === 'unknown'
    } else message.value = '该节点尚未提交准备任务'
  } catch (e) {
    showError(e, '节点操作失败')
  } finally {
    nodeBusy.value = ''
  }
}

async function quickPrepareAndEnable() {
  if (busy.value || !preview.value || issues.value.length) return
  if (dirty.value || isNew) {
    await save()
    if (failed.value || isNew) return
  }
  const availableNode = nodes.value.find(n => n.available)
  if (!availableNode) {
    showError(null, '当前没有可用的在线 Compose 节点')
    return
  }
  if (!ready.value) {
    await prepare(availableNode.id, 'prepare')
    if (failed.value) return
    await prepare(availableNode.id, 'status')
  }
  if (ready.value && !form.value.enabled) {
    form.value.enabled = true
    await save()
  }
}
</script>

<template>
  <div class="w-full text-[var(--ink)]">
    <NuxtLink
      class="inline-flex items-center gap-1.5 text-xs text-[var(--muted)] no-underline hover:text-[var(--ink)]"
      to="/admin/target-templates"
    >
      <ArrowLeft :size="14" />
      <span>靶场模板</span>
    </NuxtLink>

    <header class="flex flex-wrap items-center justify-between gap-4 my-5">
      <div class="flex items-center gap-3">
        <h1 class="text-2xl font-bold m-0">{{ isNew ? '新建靶场' : 'Compose 靶场' }}</h1>
        <UiBadge :variant="form.enabled ? 'success' : ready ? 'brand' : 'neutral'" dot>
          {{ form.enabled ? '已启用' : ready ? '已准备' : '未启用' }}
        </UiBadge>
        <UiBadge v-if="dirty" variant="warning">未保存修改</UiBadge>
      </div>

      <div class="flex items-center gap-3">
        <label
          v-if="!isNew"
          class="inline-flex items-center gap-2 text-xs font-medium cursor-pointer select-none px-2.5 py-1.5 rounded border border-[var(--line)] bg-[var(--paper)]"
        >
          <UiCheckbox v-model="form.enabled" :disabled="!ready && !form.enabled" />
          <span>允许学习和挑战关联</span>
        </label>

        <UiButton
          v-if="!isNew && !form.enabled && nodes.some(n => n.available)"
          type="button"
          variant="secondary"
          size="sm"
          :disabled="busy || !!nodeBusy || !preview || !!issues.length"
          @click="quickPrepareAndEnable"
        >
          <Zap :size="14" />
          <span>一键准备并启用</span>
        </UiButton>

        <UiButton
          variant="primary"
          size="sm"
          :disabled="busy || !preview || !!issues.length"
          :loading="busy"
          @click="save"
        >
          <Check :size="15" />
          <span>保存模板</span>
        </UiButton>
      </div>
    </header>

    <UiNotice v-if="message" :variant="failed ? 'error' : 'info'" class="mb-4">
      {{ message }}
    </UiNotice>

    <AdminDataLoading :pending="busy" :has-data="loaded" />

    <div
      v-if="loaded"
      class="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_340px] gap-6 items-start"
    >
      <form class="min-w-0" @submit.prevent="save">
        <fieldset :disabled="busy" class="border-0 m-0 p-0 min-w-0 space-y-6">
          <TargetSourcePanel
            v-model:settings="settings"
            v-model:variables="variables"
            v-model:clear-variables="clearVariables"
            :preview="preview"
            :busy="busy"
            :stored-keys="storedKeys"
            :checked-commit="checkedCommit"
            @upload="upload"
            @import-git="importGit"
            @check-git="checkGit"
            @import-update="importUpdate"
            @select-file="selectFile"
          />

          <section class="panel space-y-4">
            <h2 class="text-[15px] font-semibold m-0">基本信息</h2>
            <label class="block text-xs font-medium space-y-2 max-w-[440px]">
              <span>名称</span>
              <UiInput v-model="form.name" bare required maxlength="120" />
            </label>
            <label class="block text-xs font-medium space-y-2">
              <span>简介</span>
              <UiTextarea v-model="form.description" rows="2" maxlength="500" />
            </label>
            <label class="block text-xs font-medium space-y-2">
              <span>访问说明（学员启动靶机后显示在 IP 下方）</span>
              <UiTextarea
                v-model="form.accessInstructions"
                rows="3"
                maxlength="4000"
                placeholder="可填写服务路径、练习账号等教学说明。"
              />
            </label>
          </section>
        </fieldset>
      </form>

      <aside class="min-w-0">
        <TargetNodesPanel
          :is-new="isNew"
          :dirty="dirty"
          :node-busy="nodeBusy"
          :nodes="nodes"
          :preparations="preparations"
          @prepare="prepare"
        />
      </aside>
    </div>
  </div>
</template>

<style scoped>
.panel {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 24px;
}
</style>
