<script setup lang="ts">
import { useEditorGuard } from '~/composables/useEditorGuard'

interface Settings { repository: string; branch: string; directory: string; hasToken: boolean; version: number }
const props = defineProps<{ endpoint: string }>()
const emit = defineEmits<{ saved: [] }>()
const confirmAction = useConfirm()
const { data, pending, error, refresh } = useLazyFetch<{ data: Settings }>(() => props.endpoint)
const form = ref({ repository: '', branch: 'main', directory: '', token: '' })
const busy = ref(false), notice = ref(''), needsReload = ref(false), original = ref(JSON.stringify(form.value))
const dirty = computed(() => JSON.stringify(form.value) !== original.value)
function apply(value: Settings) {
  form.value = { repository: value.repository, branch: value.branch, directory: value.directory, token: '' }
  original.value = JSON.stringify(form.value)
}
watch(data, value => { if (value && !busy.value && !dirty.value) apply(value.data) }, { immediate: true })
const { canLeave } = useEditorGuard({ dirty, busy, title: '放弃仓库配置修改？', description: '未保存的仓库地址或凭据修改将丢失。' })
defineExpose({ canLeave })

async function readSettings() {
  await refresh()
  if (error.value || !data.value) throw Error('仓库配置读取失败')
  apply(data.value.data)
  needsReload.value = false
}
async function reload() {
  if (busy.value) return
  busy.value = true
  try { await readSettings();notice.value = '' }
  catch { notice.value = '仓库配置读取失败，请重试' }
  finally { busy.value = false }
}
async function save(clearToken = false) {
  if (busy.value || !data.value || needsReload.value || (clearToken && dirty.value)) return
  if (clearToken && !(await confirmAction({ title: '清除资料仓库 Token？', description: '已导入资料不受影响，私有仓库再次导入前需重新设置凭据。', confirmLabel: '清除 Token' }))) return
  if (busy.value || (clearToken && dirty.value)) return
  busy.value = true
  notice.value = ''
  try {
    const source = clearToken ? { ...data.value.data, token: '', clearToken: true } : { ...form.value }
    await $fetch(props.endpoint, { method: 'PUT', body: { ...source, version: data.value.data.version }, retry: false })
    form.value.token = ''
    original.value = JSON.stringify(form.value)
    needsReload.value = true
    await readSettings()
    emit('saved')
  } catch (e: any) {
    notice.value = needsReload.value ? '配置已保存，但最新数据读取失败。请重新读取后继续。' : e?.data?.message || '配置保存失败，输入已保留'
  } finally { busy.value = false }
}
</script>

<template>
  <section class="panel repository-settings" aria-label="资料仓库设置">
    <h2>GitHub 仓库</h2>
    <UiBusyIndicator v-if="pending && !data" label="仓库设置加载" />
    <UiNotice v-if="error || notice" :variant="needsReload ? 'warning' : 'error'" class="mb-4">
      {{ notice || '无法读取仓库设置' }}
      <UiButton v-if="error || needsReload" variant="secondary" size="xs" :disabled="busy" @click="reload">重新读取</UiButton>
    </UiNotice>
    <form v-if="data" @submit.prevent="save()">
      <fieldset :disabled="busy || needsReload">
        <div class="repo-fields">
          <label>仓库<input v-model="form.repository" required maxlength="240" placeholder="owner/repository" /></label>
          <label>分支<input v-model="form.branch" required maxlength="200" /></label>
          <label>资料目录<input v-model="form.directory" maxlength="500" placeholder="references" /></label>
        </div>
        <label class="token-field">只读 Token<input v-model="form.token" type="password" autocomplete="new-password" maxlength="4096" :placeholder="data.data.hasToken ? '已设置，留空保留' : '公开仓库可留空'" /></label>
        <div class="repo-footer">
          <p>与教程使用独立扫描目录。Token 加密保存，不回显。</p>
          <div class="actions">
            <UiButton v-if="data.data.hasToken" variant="danger" size="sm" :disabled="busy || dirty" @click="save(true)">清除 Token</UiButton>
            <UiButton variant="secondary" size="sm" :disabled="!dirty || busy" @click="apply(data.data)">取消修改</UiButton>
            <UiButton type="submit" size="sm" :loading="busy" :disabled="!dirty || !form.repository.trim()">保存仓库</UiButton>
          </div>
        </div>
      </fieldset>
    </form>
  </section>
</template>

<style scoped>
fieldset { margin: 0; padding: 0; border: 0; min-width: 0; }
.repo-fields { display: grid; grid-template-columns: minmax(260px, 2fr) minmax(120px, .7fr) minmax(180px, 1fr); gap: 20px; }
label { display: grid; gap: 9px; font-size: 12px; color: var(--muted); }
input { width: 100%; height: 38px; padding: 8px 11px; border: 1px solid var(--line); border-radius: 6px; background: var(--soft); color: var(--ink); font-size: 13px; }
.token-field { max-width: 460px; margin-top: 20px; }
.repo-footer { display: flex; justify-content: space-between; align-items: end; gap: 24px; margin-top: 24px; }
.repo-footer p { margin: 0; color: var(--muted); font-size: 12px; }
.actions { display: flex; align-items: center; gap: 10px; }
</style>
