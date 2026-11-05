<script setup lang="ts">
import { ref, computed } from 'vue'
import { Upload, FileArchive, Github, Download, Trash2, RefreshCw } from 'lucide-vue-next'
import type { ComposePreview, ComposeSettings } from '~/shared/compose-labs'

const props = defineProps<{
  preview: ComposePreview | null
  settings: ComposeSettings
  busy: boolean
  storedKeys: string[]
  variables: Record<string, string>
  clearVariables: string[]
  checkedCommit: string
}>()

const emit = defineEmits<{
  upload: [event: Event]
  importGit: [payload: { repository: string; branch: string; directory: string; token: string }]
  checkGit: []
  importUpdate: []
  selectFile: [file: string]
  'update:settings': [value: ComposeSettings]
  'update:variables': [value: Record<string, string>]
  'update:clearVariables': [value: string[]]
}>()

const gitOpen = ref(false)
const gitForm = ref({ repository: '', branch: 'main', directory: '', token: '' })

const issues = computed(() => [
  ...(props.preview?.issues || []),
  ...(props.preview?.missingFiles || []).map(f => `缺少配套文件／目录：${f}`)
])

const composeFileOptions = computed(
  () => props.preview?.composeFiles.map(file => ({ value: file, label: file })) || []
)

const entryServiceObj = computed(() =>
  props.preview?.services.find(s => s.name === props.settings.entryService)
)

const networkOptions = computed(() =>
  (entryServiceObj.value?.networks || []).map(name => ({ value: name, label: name }))
)

const hasPendingVariableEdits = computed(
  () =>
    Object.keys(props.variables).some(k => props.variables[k]) || props.clearVariables.length > 0
)

function parseGitHubInput(raw: string) {
  gitForm.value.repository = raw
  const trimmed = raw.trim()
  const treeMatch = trimmed.match(
    /^https?:\/\/github\.com\/([^/]+\/[^/]+?)(?:\.git)?\/tree\/([^/]+)(?:\/(.*))?$/i
  )
  if (treeMatch) {
    gitForm.value.repository = treeMatch[1]!
    gitForm.value.branch = treeMatch[2] || 'main'
    gitForm.value.directory = (treeMatch[3] || '').replace(/\/+$/, '')
    return
  }
  const blobMatch = trimmed.match(
    /^https?:\/\/github\.com\/([^/]+\/[^/]+?)(?:\.git)?\/blob\/([^/]+)\/(.*)$/i
  )
  if (blobMatch) {
    gitForm.value.repository = blobMatch[1]!
    gitForm.value.branch = blobMatch[2] || 'main'
    const parts = (blobMatch[3] || '').split('/')
    parts.pop()
    gitForm.value.directory = parts.join('/')
  }
}

function submitGit() {
  if (!gitForm.value.repository.trim()) return
  emit('importGit', { ...gitForm.value })
  gitForm.value.token = ''
  gitOpen.value = false
}

function setEntryService(serviceName: string) {
  const svc = props.preview?.services.find(s => s.name === serviceName)
  if (!svc) return
  const nextPort =
    (svc.ports?.find(p => p.protocol === 'tcp') || svc.ports?.[0])?.port || props.settings.entryPort
  const nextNet = svc.networks.includes(props.settings.entryNetwork)
    ? props.settings.entryNetwork
    : svc.networks[0] || 'default'
  emit('update:settings', {
    ...props.settings,
    entryService: serviceName,
    entryPort: nextPort,
    entryNetwork: nextNet
  })
}

function updatePort(port: string | number) {
  emit('update:settings', {
    ...props.settings,
    entryPort: Number(port) || 8080
  })
}

function updateNetwork(network: string) {
  emit('update:settings', {
    ...props.settings,
    entryNetwork: network
  })
}

function updateVar(key: string, val: string | number) {
  emit('update:variables', {
    ...props.variables,
    [key]: String(val)
  })
}

function toggleClearVar(key: string) {
  const next = props.clearVariables.includes(key)
    ? props.clearVariables.filter(k => k !== key)
    : [...props.clearVariables, key]
  emit('update:clearVariables', next)
}
</script>

<template>
  <section class="panel space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-[15px] font-semibold m-0">部署文件与服务</h2>
        <p class="text-xs text-[var(--muted)] m-0 mt-1">
          支持上传单文件 Compose、ZIP 部署包，或直接粘贴 Vulhub 等开源靶场 GitHub 目录链接。
        </p>
      </div>
      <div class="flex items-center gap-2">
        <UiButton type="button" variant="secondary" size="xs" @click="gitOpen = !gitOpen">
          <Github :size="14" />
          <span>从 GitHub 导入</span>
        </UiButton>
        <label class="file-button">
          <Upload :size="14" />
          <span>{{ preview ? '替换本地文件' : '选择文件' }}</span>
          <input
            type="file"
            accept=".zip,.yml,.yaml"
            :disabled="busy"
            @change="emit('upload', $event)"
          />
        </label>
      </div>
    </div>

    <!-- 本地包或未上传时的状态条 -->
    <div
      v-if="!preview?.source && !gitOpen"
      class="flex items-center gap-3.5 rounded-lg border border-dashed border-[var(--line)] bg-[var(--bg)] px-4 py-3.5"
    >
      <FileArchive :size="20" class="text-[var(--muted)] shrink-0" />
      <div class="min-w-0 flex-1">
        <strong class="text-xs block">
          {{
            preview
              ? `${preview.services.length} 个服务 · ${preview.files} 个文件`
              : '选择 Compose 文件 (.yml) 或 ZIP 部署包'
          }}
        </strong>
        <span class="text-xs text-[var(--muted)]">
          {{
            preview
              ? `当前入口：${settings.entryService}:${settings.entryPort}`
              : '也可点击右上角「从 GitHub 导入」直接拉取开源靶场仓库目录'
          }}
        </span>
      </div>
    </div>

    <!-- GitHub 导入折叠面板 -->
    <div
      v-if="gitOpen"
      class="rounded-lg border border-[var(--line)] bg-[var(--bg)] p-4 space-y-3.5"
    >
      <label class="block text-xs font-medium space-y-1.5">
        <span>GitHub 仓库</span>
        <UiInput
          :model-value="gitForm.repository"
          bare
          placeholder="粘贴 GitHub 目录链接（如 https://github.com/vulhub/vulhub/tree/master/...）或 owner/repo"
          @update:model-value="parseGitHubInput"
        />
      </label>
      <div class="grid grid-cols-1 sm:grid-cols-[160px_minmax(0,1fr)] gap-3">
        <label class="block text-xs font-medium space-y-1.5">
          <span>分支 / 标签</span>
          <UiInput v-model="gitForm.branch" bare placeholder="main" />
        </label>
        <label class="block text-xs font-medium space-y-1.5">
          <span>仓库内目录</span>
          <UiInput v-model="gitForm.directory" bare placeholder="留空读取仓库根目录" />
        </label>
      </div>
      <label class="block text-xs font-medium space-y-1.5">
        <span>只读 Token（公开仓库留空）</span>
        <UiInput
          v-model="gitForm.token"
          bare
          type="password"
          autocomplete="new-password"
          placeholder="公开仓库可留空；私有仓库填只读 Token"
        />
      </label>
      <div class="flex items-center justify-between gap-3 pt-1">
        <span class="text-xs text-[var(--muted)]"
          >{{
            preview
              ? `${preview.services.length} 个服务 · ${preview.files} 个文件`
              : '导入后自动识别服务与端口'
          }}
        </span>
        <div class="flex items-center gap-2">
          <UiButton type="button" variant="ghost" size="xs" @click="gitOpen = false">收起</UiButton>
          <UiButton
            type="button"
            variant="primary"
            size="sm"
            :disabled="!gitForm.repository.trim()"
            @click="submitGit"
          >
            <Download :size="14" />
            <span>导入并预览</span>
          </UiButton>
        </div>
      </div>
    </div>

    <!-- 已绑定的 GitHub 来源信息条 -->
    <div
      v-if="preview?.source"
      class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-[var(--line)] bg-[var(--bg)] px-4 py-3 text-xs"
    >
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2 flex-wrap">
          <strong>{{ preview.source.repository }}</strong>
          <UiBadge variant="neutral">{{ preview.source.branch }}</UiBadge>
          <code class="text-[11px] text-[var(--muted)]" :title="preview.source.commit">
            {{ preview.source.commit.slice(0, 12) }}
          </code>
          <span class="text-[var(--muted)]">
            · {{ preview.services.length }} 个服务 · {{ preview.files }} 个文件
          </span>
        </div>
        <p class="text-[var(--muted)] m-0 mt-1 truncate">
          目录：{{ preview.source.directory || '仓库根目录' }}
        </p>
      </div>
      <UiButton
        variant="secondary"
        size="xs"
        type="button"
        :disabled="hasPendingVariableEdits"
        @click="emit('checkGit')"
      >
        <RefreshCw :size="12" />
        <span>检查更新</span>
      </UiButton>
      <div
        v-if="checkedCommit"
        class="w-full flex items-center justify-between pt-2.5 mt-1 border-t border-[var(--line)]"
      >
        <span>
          发现新提交 <code>{{ checkedCommit.slice(0, 12) }}</code>
        </span>
        <UiButton
          variant="primary"
          size="xs"
          type="button"
          :disabled="hasPendingVariableEdits"
          @click="emit('importUpdate')"
        >
          确认导入更新
        </UiButton>
      </div>
    </div>

    <!-- 多 Compose 文件入口选择（仅包内有多个 compose 文件时才出现） -->
    <div v-if="preview && preview.composeFiles.length > 1" class="max-w-xs">
      <label class="block text-xs font-medium space-y-1.5">
        <span>部署入口</span>
        <UiSelect
          :model-value="settings.file"
          aria-label="部署入口"
          :options="composeFileOptions"
          @update:model-value="emit('selectFile', $event)"
        />
      </label>
    </div>

    <!-- 校验阻塞问题提示 -->
    <UiNotice v-if="issues.length" variant="error">
      <ul class="m-0 pl-4 space-y-1">
        <li v-for="issue in issues" :key="issue">{{ issue }}</li>
      </ul>
    </UiNotice>

    <!-- 服务与内联访问入口表格（彻底替代原右栏孤立的“访问入口”面板） -->
    <div v-if="preview" class="border border-[var(--line)] rounded-lg overflow-x-auto">
      <table class="w-full border-collapse text-xs text-left">
        <thead>
          <tr class="border-b border-[var(--line)] bg-[var(--soft)] text-[var(--muted)]">
            <th class="py-2.5 px-3.5 font-medium">服务</th>
            <th class="py-2.5 px-3.5 font-medium">镜像／构建</th>
            <th class="py-2.5 px-3.5 font-medium w-56">访问端口（学员连接入口）</th>
            <th v-if="preview.services.length > 1" class="py-2.5 px-3.5 font-medium w-28 text-right">
              入口
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-[var(--line)]">
          <tr v-for="service in preview.services" :key="service.name">
            <td class="py-3 px-3.5 align-middle">
              <div class="flex items-center gap-2">
                <strong class="font-semibold">{{ service.name }}</strong>
                <UiBadge
                  v-if="settings.entryService === service.name"
                  variant="brand"
                  dot
                >
                  主入口
                </UiBadge>
              </div>
            </td>
            <td class="py-3 px-3.5 align-middle">
              <code class="text-[11px]">{{
                service.build ? service.build.dockerfile : service.image
              }}</code>
              <small v-if="service.build" class="build-context block text-[var(--muted)] mt-1">
                本地构建 · {{ service.build.context
                }}<span v-if="service.build.target"> · {{ service.build.target }}</span>
              </small>
            </td>
            <td class="py-3 px-3.5 align-middle">
              <div
                v-if="settings.entryService === service.name"
                class="flex items-center gap-2"
              >
                <label class="sr-only" for="entry-port-input">端口</label>
                <UiInput
                  id="entry-port-input"
                  :model-value="settings.entryPort"
                  bare
                  size="sm"
                  type="number"
                  min="1"
                  max="65535"
                  aria-label="端口"
                  class="!w-24"
                  @update:model-value="updatePort"
                />
                <!-- 仅当该入口服务跨多个自定义网络时才显示网段下拉框 -->
                <UiSelect
                  v-if="networkOptions.length > 1"
                  :model-value="settings.entryNetwork"
                  aria-label="接入网络"
                  :options="networkOptions"
                  class="!w-28"
                  @update:model-value="updateNetwork"
                />
                <span v-else class="text-[var(--muted)]">/ TCP</span>
              </div>
              <span v-else class="text-[var(--muted)]">
                {{
                  service.ports?.length
                    ? service.ports.map(p => `${p.port}/${p.protocol}`).join(', ')
                    : '内部服务'
                }}
              </span>
            </td>
            <td v-if="preview.services.length > 1" class="py-3 px-3.5 align-middle text-right">
              <UiButton
                v-if="settings.entryService !== service.name"
                type="button"
                variant="secondary"
                size="xs"
                @click="setEntryService(service.name)"
              >
                设为入口
              </UiButton>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 部署变量（仅当 Compose 中声明了 ${VAR} 时才显示） -->
    <div
      v-if="preview?.variables.length"
      class="pt-4 border-t border-[var(--line)] space-y-3"
    >
      <div>
        <h3 class="text-xs font-semibold m-0">部署变量</h3>
        <p class="text-xs text-[var(--muted)] m-0 mt-0.5">
          加密保存且不回显；留空则保留已有值或使用 Compose 默认值。
        </p>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <label
          v-for="key in preview.variables"
          :key="key"
          class="block text-xs font-medium space-y-1.5"
        >
          <span class="flex items-center justify-between gap-2">
            <code>{{ key }}</code>
            <UiButton
              v-if="storedKeys.includes(key)"
              variant="secondary"
              size="xs"
              type="button"
              :aria-label="`清除 ${key} 的已保存值`"
              @click="toggleClearVar(key)"
            >
              <Trash2 :size="12" />
              <span>{{ clearVariables.includes(key) ? '待清除' : '清除' }}</span>
            </UiButton>
          </span>
          <UiInput
            :model-value="variables[key] || ''"
            bare
            type="password"
            autocomplete="new-password"
            :placeholder="
              clearVariables.includes(key)
                ? '保存后清除，使用默认值'
                : storedKeys.includes(key)
                  ? '已设置，留空保留'
                  : '输入变量值'
            "
            @update:model-value="updateVar(key, $event)"
          />
        </label>
      </div>
    </div>
  </section>
</template>

<style scoped>
.panel {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 24px;
}

.file-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--line);
  background: var(--paper);
  color: var(--ink);
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  position: relative;
  overflow: hidden;
}

.file-button:hover {
  background: var(--soft);
}

.file-button input {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
  width: 100%;
}
</style>
