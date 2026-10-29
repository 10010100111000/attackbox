<script setup lang="ts">
const confirmAction = useConfirm()
import { computed, ref, watch } from 'vue'
import { ArrowLeft, GitBranch, Download, RefreshCw, CheckCircle2, FolderGit2 } from 'lucide-vue-next'
import type { RepoScanReport, RepoModuleEntry, SyncModuleResult } from '~/shared/content-sync'

definePageMeta({ layout: 'admin' })
useHead({ title: 'Git 教程同步 — AttackBox' })

interface Settings {
  repository: string
  branch: string
  directory: string
  hasToken: boolean
  version: number
}

const { data: settings, error: settingsError, refresh } = useLazyFetch<{ data: Settings }>(
  '/api/admin/content/repository',
  { key: 'admin:content:repository' },
)

const form = ref({ repository: '', branch: 'main', directory: '', token: '' })
const busy = ref('')
const syncingPath = ref('')
const message = ref('')
const scan = ref<RepoScanReport | null>(null)

watch(
  settings,
  value => {
    if (value) {
      form.value = {
        repository: value.data.repository,
        branch: value.data.branch,
        directory: value.data.directory,
        token: '',
      }
    }
  },
  { immediate: true },
)

const dirty = computed(
  () =>
    !!settings.value &&
    (form.value.repository !== settings.value.data.repository ||
      form.value.branch !== settings.value.data.branch ||
      form.value.directory !== settings.value.data.directory ||
      !!form.value.token),
)

const changedModules = computed(() =>
  (scan.value?.modules || []).filter(m => m.status === 'new' || m.status === 'modified'),
)

async function perform(label: string, task: () => Promise<void>) {
  if (busy.value) return
  busy.value = label
  message.value = ''
  try {
    await task()
  } catch (e: any) {
    message.value = e?.data?.message || e?.data?.statusMessage || '操作失败，请检查仓库配置或重试'
  } finally {
    busy.value = ''
    syncingPath.value = ''
  }
}

function save() {
  void perform('保存配置', async () => {
    await $fetch('/api/admin/content/repository', {
      method: 'PUT',
      body: { ...form.value, version: settings.value?.data.version || 0 },
    })
    form.value.token = ''
    await refresh()
    message.value = '仓库配置已保存'
  })
}

async function clearToken() {
  if (!(await confirmAction({title:'清除只读 Token？',description:'已经导入的教程不受影响。再次同步私有仓库前需要重新配置凭据。',confirmLabel:'清除 Token'}))) return
  void perform('清除凭据', async () => {
    await $fetch('/api/admin/content/repository', {
      method: 'PUT',
      body: { ...form.value, token: '', version: settings.value?.data.version, clearToken: true },
    })
    await refresh()
    message.value = '凭据已清除'
  })
}

function testConnection() {
  void perform('测试连接', async () => {
    const response = await $fetch<{ data: { commit: string } }>('/api/admin/content/test', {
      method: 'POST',
    })
    message.value = `连接成功 · 最新提交 ${response.data.commit.slice(0, 12)}`
  })
}

function scanRepo() {
  void perform('读取仓库差异', async () => {
    const response = await $fetch<{ data: RepoScanReport }>('/api/admin/content/pull', {
      method: 'POST',
    })
    scan.value = response.data
  })
}

function syncSingle(item: RepoModuleEntry, force = false) {
  syncingPath.value = item.sourcePath
  void perform(`同步 ${item.title}`, async () => {
    const response = await $fetch<{ data: SyncModuleResult }>('/api/admin/content/sync', {
      method: 'POST',
      body: { sourcePath: item.sourcePath, force },
    })
    const res = response.data.synced[0]
    clearNuxtData(k => k.startsWith('admin:') || k.startsWith('learning:'))
    const refreshed = await $fetch<{ data: RepoScanReport }>('/api/admin/content/pull', {
      method: 'POST',
    })
    scan.value = refreshed.data
    if (res) {
      const actionLabel = res.action === 'created' ? '已新增导入' : res.action === 'updated' ? '已差异更新' : '无变化'
      message.value = `${res.title}：${actionLabel}（下载图片 ${res.downloadedImages} 张，复用未变图片 ${res.reusedImages} 张）`
    }
  })
}

function syncAllChanged() {
  if (!changedModules.value.length) return
  void perform('同步全部变更模块', async () => {
    const response = await $fetch<{ data: SyncModuleResult }>('/api/admin/content/sync', {
      method: 'POST',
      body: {},
    })
    clearNuxtData(k => k.startsWith('admin:') || k.startsWith('learning:'))
    const refreshed = await $fetch<{ data: RepoScanReport }>('/api/admin/content/pull', {
      method: 'POST',
    })
    scan.value = refreshed.data
    message.value = `已同步 ${response.data.synced.length} 个模块`
  })
}
</script>

<template>
  <div class="git-content-page">
    <NuxtLink class="back" to="/admin/modules">
      <ArrowLeft :size="14" />学习模块
    </NuxtLink>

    <header class="page-header">
      <div>
        <h1>Git 教程同步</h1>
        <p>按模块目录直接读取 Git 差异，仅下载有变化的 Markdown 与新增图片，直接入库生效。</p>
      </div>
      <a class="button" href="/api/admin/content/template" download="教程模板.md">
        <Download :size="15" />下载 MD 模板
      </a>
    </header>

    <UiNotice v-if="message" variant="info" class="mb-4">{{ message }}</UiNotice>
    <UiNotice v-if="settingsError" variant="error" class="mb-4">
      <div class="flex items-center justify-between gap-3 w-full">
        <span>配置读取失败。</span>
        <UiButton variant="secondary" size="xs" type="button" @click="refresh()">重新读取</UiButton>
      </div>
    </UiNotice>

    <section class="panel">
      <h2><GitBranch :size="17" />仓库设置</h2>
      <form @submit.prevent="save">
        <fieldset :disabled="!!busy || !settings">
          <div class="settings-grid">
            <label>
              GitHub 私有仓库
              <input v-model="form.repository" placeholder="owner/repository" autocomplete="off" required />
            </label>
            <label>
              分支
              <input v-model="form.branch" placeholder="main" required />
            </label>
            <label>
              基础目录
              <input v-model="form.directory" placeholder="留空为仓库根目录（自动扫描子文件夹）" />
            </label>
            <label class="token-field">
              只读 Token
              <span>{{ settings?.data.hasToken ? '已配置，留空保持不变' : '未配置' }}</span>
              <input
                v-model="form.token"
                type="password"
                autocomplete="new-password"
                placeholder="仅授权当前仓库的 Contents: Read-only"
              />
            </label>
          </div>
          <div class="actions flex items-center gap-3">
            <UiButton variant="secondary" size="sm" type="submit">保存配置</UiButton>
            <UiButton
              variant="secondary"
              size="sm"
              type="button"
              :disabled="dirty || !settings?.data.hasToken"
              @click="testConnection"
            >
              测试连接
            </UiButton>
            <UiButton
              variant="primary"
              size="sm"
              type="button"
              :disabled="dirty || !settings?.data.hasToken"
              @click="scanRepo"
            >
              <RefreshCw :size="14" />
              <span>读取仓库差异</span>
            </UiButton>
            <UiButton v-if="settings?.data.hasToken" variant="ghost" size="sm" type="button" @click="clearToken">
              清除凭据
            </UiButton>
            <UiBusyIndicator v-if="busy" inline :label="busy" />
          </div>
        </fieldset>
      </form>
    </section>

    <section v-if="scan" class="panel">
      <div class="preview-head">
        <div>
          <h2><FolderGit2 :size="17" />仓库模块差异列表</h2>
          <p>
            Commit <code>{{ scan.commit.slice(0, 12) }}</code> · 共发现 {{ scan.modules.length }} 个模块（{{
              changedModules.length
            }}
            个待导入或有更新）
          </p>
        </div>
        <UiButton
          v-if="changedModules.length > 0"
          variant="primary"
          size="sm"
          type="button"
          :disabled="!!busy"
          @click="syncAllChanged"
        >
          <CheckCircle2 :size="15" />
          <span>一键同步全部变更 ({{ changedModules.length }})</span>
        </UiButton>
      </div>

      <div class="module-list">
        <div v-for="item in scan.modules" :key="item.sourcePath" class="module-row">
          <div class="module-meta">
            <div class="module-title-line flex items-center gap-2">
              <strong>{{ item.title }}</strong>
              <UiBadge :variant="item.status === 'new' ? 'brand' : item.status === 'modified' ? 'warning' : 'success'" dot>
                {{ item.status === 'new' ? '新增未导入' : item.status === 'modified' ? '有内容更新' : '已是最新' }}
              </UiBadge>
              <UiBadge v-if="item.moduleStatus" :variant="item.moduleStatus === 'published' ? 'success' : 'neutral'">
                {{ item.moduleStatus === 'published' ? '线上已发布' : '未发布草稿' }}
              </UiBadge>
            </div>
            <div class="module-sub-line">
              <code>{{ item.sourcePath }}</code>
              <span v-if="item.changedReasons.length" class="diff-reason">
                · {{ item.changedReasons.join(' / ') }}
              </span>
              <span v-else-if="item.syncedCommit" class="synced-info">
                · 已同步于 {{ item.syncedCommit.slice(0, 8) }}
              </span>
            </div>
          </div>

          <div class="module-actions flex items-center gap-2">
            <NuxtLink
              v-if="item.moduleId"
              class="button"
              :to="`/admin/modules/${item.moduleId}`"
            >
              模块详情
            </NuxtLink>
            <UiButton
              v-if="item.status === 'new'"
              variant="primary"
              size="xs"
              type="button"
              :disabled="!!busy"
              @click="syncSingle(item)"
            >
              直接导入
            </UiButton>
            <UiButton
              v-else-if="item.status === 'modified'"
              variant="primary"
              size="xs"
              type="button"
              :disabled="!!busy"
              @click="syncSingle(item)"
            >
              同步更新
            </UiButton>
            <UiButton
              v-else
              variant="secondary"
              size="xs"
              type="button"
              :disabled="!!busy"
              @click="syncSingle(item, true)"
            >
              重新同步
            </UiButton>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  font-size: 12px;
  text-decoration: none;
  margin-bottom: 18px;
}
.page-header,
.preview-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 24px;
}
h1 {
  font-size: 26px;
  font-weight: 650;
  margin: 0;
}
h2 {
  font-size: 16px;
  font-weight: 600;
  margin: 0 0 8px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.page-header p,
.preview-head p {
  font-size: 12px;
  color: var(--muted);
  line-height: 1.8;
  margin: 4px 0 0;
}
.panel {
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--paper);
  padding: 24px;
  margin-bottom: 20px;
}
fieldset {
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
}
.settings-grid {
  display: grid;
  grid-template-columns: 2fr 1fr 1.2fr;
  gap: 20px;
}
.token-field {
  grid-column: 1 / 3;
}
.token-field span {
  font-size: 11px;
  color: var(--muted);
  font-weight: 400;
}
label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
  font-weight: 550;
}
input {
  height: 38px;
  width: 100%;
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--bg);
  color: var(--ink);
  font-size: 12px;
}
.actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 20px;
}
.button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 8px 13px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--paper);
  color: var(--ink);
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
  text-decoration: none;
}
.primary {
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  border-color: transparent;
  box-shadow: var(--btn-primary-shadow);
  font-weight: 600;
}
.primary:hover:not(:disabled) {
  background: var(--btn-primary-hover);
}
button:disabled {
  opacity: 0.45;
  cursor: default;
}
.text-button {
  border: 0;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  cursor: pointer;
}
.notice {
  padding: 12px 16px;
  background: var(--soft);
  border-radius: 6px;
  font-size: 12px;
  margin-bottom: 18px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.module-list {
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--line);
}
.module-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 4px;
  border-bottom: 1px solid var(--line);
}
.module-row:last-child {
  border-bottom: 0;
  padding-bottom: 0;
}
.module-meta {
  display: flex;
  flex-direction: column;
  gap: 5px;
}
.module-title-line {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}
.module-sub-line {
  font-size: 12px;
  color: var(--muted);
}
.module-sub-line code {
  font-size: 11px;
}
.diff-reason {
  color: var(--ink);
}
.status-badge,
.pub-badge {
  font-size: 11px;
  padding: 2px 7px;
  border-radius: 4px;
  font-weight: 500;
}
.badge-new,
.badge-modified {
  background: var(--success-bg);
  color: var(--success-ink);
}
.badge-ok,
.pub-badge {
  background: var(--soft);
  color: var(--muted);
}
.module-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
