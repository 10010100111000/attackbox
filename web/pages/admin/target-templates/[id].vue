<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { ArrowLeft, Plus, X } from 'lucide-vue-next'
import { onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'
import {
  parseTargetTemplate,
  TemplateValidationError,
  type TargetTemplate,
  type TargetTemplateInput,
} from '~/shared/target-templates'

definePageMeta({ layout: 'admin', key: route => route.fullPath })
useHead({ title: '编辑靶机模板 — AttackBox' })

const route = useRoute()
const id = String(route.params.id)
const isNew = id === 'new'
const form = ref<TargetTemplateInput>({
  name: '',
  description: '',
  image: '',
  sourceUrl: '',
  ports: [],
  accessInstructions: '',
  enabled: true,
})
const imageNodes = ref<{nodeId:string;state:string;error?:string}[]>([])
const imageBusy = ref(false)
let imageTimer: ReturnType<typeof setTimeout> | undefined
async function imageAction(action: 'status' | 'prepare' = 'status') {
 if (isNew || imageBusy.value) return
 imageBusy.value = true
 clearTimeout(imageTimer)
 try {
  imageNodes.value = (await $fetch<{data: typeof imageNodes.value}>(`/api/admin/target-templates/${id}/image`, {method:'POST',body:{action}})).data
  if (imageNodes.value.some(n=>n.state==='preparing')) imageTimer=setTimeout(()=>imageAction(),3000)
 } catch { notice.value='镜像状态查询失败，请重试'; failure.value=true }
 finally { imageBusy.value=false }
}
onBeforeUnmount(()=>clearTimeout(imageTimer))
const imageLabels: Record<string,string> = {ready:'就绪',missing:'未准备',preparing:'正在拉取',failed:'准备失败',unknown:'未知'}
const original = ref(JSON.stringify(form.value))
const version = ref(0)
const loaded = ref(isNew)
const busy = ref(!isNew)
const notice = ref('')
const failure = ref(false)
const conflict = ref(false)
const errors = ref<Record<string, string>>({})

const dirty = computed(() => loaded.value && JSON.stringify(form.value) !== original.value)

function canLeave() {
  return busy.value && loaded.value ? false : !dirty.value || confirm('有未保存的修改，确定离开？')
}
onBeforeRouteLeave(canLeave)
onBeforeRouteUpdate(canLeave)

function beforeUnload(e: BeforeUnloadEvent) {
  if (dirty.value) {
    e.preventDefault()
    e.returnValue = ''
  }
}

onMounted(() => window.addEventListener('beforeunload', beforeUnload))
onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))

function apply(t: TargetTemplate) {
  form.value = {
    name: t.name,
    description: t.description,
    image: t.image,
    sourceUrl: t.sourceUrl,
    ports: t.ports,
    accessInstructions: t.accessInstructions,
    enabled: t.enabled,
  }
  version.value = t.lockVersion
  original.value = JSON.stringify(form.value)
  loaded.value = true
  conflict.value = false
}

const request = useRequestFetch()

async function load() {
  busy.value = true
  failure.value = false
  notice.value = ''
  try {
    apply((await request<{ data: TargetTemplate }>(`/api/admin/target-templates/${id}`)).data)
  } catch {
    failure.value = true
    notice.value = '无法读取模板，请检查登录状态后重试。'
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
 if (!isNew) {
  await load()
  if (loaded.value) void imageAction()
  if (loaded.value && route.query.created === '1') notice.value = '模板已创建'
 }
})

async function reload() {
  if (dirty.value && !confirm('重新读取会放弃当前未保存输入，确定继续？')) return
  await load()
}

async function save() {
  errors.value = {}
  notice.value = ''
  failure.value = false
  let parsed: TargetTemplateInput
  try {
    parsed = parseTargetTemplate(form.value)
  } catch (e) {
    if (e instanceof TemplateValidationError) {
      errors.value = { [e.field]: e.message }
      notice.value = '请检查标记的配置项'
      failure.value = true
      await nextTick()
      document.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
      return
    }
    throw e
  }

  busy.value = true
  try {
    const response = await $fetch<{ data: TargetTemplate }>(
      isNew ? '/api/admin/target-templates' : `/api/admin/target-templates/${id}`,
      {
        method: isNew ? 'POST' : 'PUT',
        body: { ...parsed, lockVersion: version.value },
      }
    )
    apply(response.data)
    clearNuxtData('admin:target-templates')
    if (isNew) {
      busy.value = false
      await navigateTo({ path: `/admin/target-templates/${response.data.id}`, query: { created: '1' } })
    } else {
      notice.value = '模板已保存'
    }
  } catch (e: any) {
    failure.value = true
    conflict.value = e?.statusCode === 409 || e?.response?.status === 409
    notice.value = e?.data?.message || '保存失败，当前输入已保留，请重试'
    if (e?.data?.data?.field) {
      errors.value = { [e.data.data.field]: notice.value }
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="template-editor">
    <AdminDataLoading :pending="busy" :has-data="loaded" />
    <NuxtLink to="/admin/target-templates" class="back">
      <ArrowLeft :size="14" aria-hidden="true" />
      <span>靶机模板</span>
    </NuxtLink>

    <header class="editor-header">
      <div class="title-group">
        <h1>{{ isNew ? '新建模板' : form.name || '编辑模板' }}</h1>
        <span class="status-badge">{{ dirty ? '有未保存修改' : '静态环境配置' }}</span>
      </div>
      <button
        class="primary"
        type="submit"
        form="template-form"
        :disabled="busy || !loaded || conflict"
      >
        {{ busy ? '处理中…' : '保存模板' }}
      </button>
    </header>

    <div v-if="notice" class="notice" :class="{ failure }" role="status">
      <span>{{ notice }}</span>
      <button v-if="conflict || !loaded" type="button" :disabled="busy" @click="reload">
        重新读取
      </button>
    </div>

    <form v-if="loaded" id="template-form" @submit.prevent="save">
      <fieldset :disabled="busy">
        <div class="editor-grid">
          <section class="panel">
            <h2>基础信息</h2>
            <label>
              名称 <span class="required">*</span>
              <input v-model="form.name" maxlength="120" :aria-invalid="!!errors.name" />
              <small v-if="errors.name" class="error">{{ errors.name }}</small>
            </label>

            <label>
              用途说明
              <textarea
                v-model="form.description"
                rows="3"
                maxlength="500"
                placeholder="说明环境适合哪些教学或挑战任务"
              />
            </label>

            <h2 class="section-title">镜像</h2>
            <label>
              镜像引用 <span class="required">*</span>
              <input
                v-model="form.image"
                maxlength="512"
                placeholder="例如 ghcr.io/example/lab:1.0"
                :aria-invalid="!!errors.image"
              />
              <small :class="{ error: errors.image }">
                {{ errors.image || '填写明确的 tag 或 sha256 digest；这里只保存配置，不拉取或验证镜像。' }}
              </small>
            </label>

            <div v-if="!isNew" class="image-preparation">
              <p class="hint">镜像按节点准备。保存模板后点击准备；学员启动不会自动下载。</p>
              <p v-if="!imageNodes.length" class="hint">尚无可用 Docker 节点或未读取状态。</p>
              <p v-for="node in imageNodes" :key="node.nodeId" role="status">{{ node.nodeId }} · {{ imageLabels[node.state] || node.state }} <small v-if="node.error">{{ node.error }}</small></p>
              <button type="button" :disabled="imageBusy || dirty" @click="imageAction('prepare')">准备镜像 / 重试</button>
              <button type="button" :disabled="imageBusy || dirty" @click="imageAction()">刷新状态</button>
              <small v-if="dirty">请先保存修改，再准备镜像。</small>
            </div>
            <label>
              来源项目
              <input
                v-model="form.sourceUrl"
                maxlength="1000"
                placeholder="https://github.com/owner/project"
                :aria-invalid="!!errors.sourceUrl"
              />
              <small>仅记录项目来源，不下载源码或执行构建脚本。</small>
              <small v-if="errors.sourceUrl" class="error">{{ errors.sourceUrl }}</small>
            </label>

            <label class="toggle">
              <input v-model="form.enabled" type="checkbox" />
              <span>启用模板，允许后续学习与挑战关联</span>
            </label>
          </section>

          <aside class="panel">
            <h2>
              <span>服务端口</span>
              <span class="muted">{{ form.ports.length }} / 20</span>
            </h2>
            <p class="hint">填写容器内端口，不配置公网映射。</p>

            <div v-for="(port, index) in form.ports" :key="index" class="port-row">
              <input
                v-model.number="port.port"
                type="number"
                min="1"
                max="65535"
                :aria-label="`端口 ${index + 1}`"
                :aria-invalid="!!errors.ports"
              />
              <select v-model="port.protocol" :aria-label="`协议 ${index + 1}`">
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
              </select>
              <button
                type="button"
                class="remove-port-btn"
                :aria-label="`移除端口 ${index + 1}`"
                @click="form.ports.splice(index, 1)"
              >
                <X :size="14" aria-hidden="true" />
              </button>
            </div>

            <small v-if="errors.ports" class="error">{{ errors.ports }}</small>

            <button
              type="button"
              class="add-port"
              :disabled="form.ports.length >= 20"
              @click="form.ports.push({ port: 80, protocol: 'tcp' })"
            >
              <Plus :size="14" aria-hidden="true" />
              <span>添加端口</span>
            </button>

            <h2 class="section-title">访问说明</h2>
            <label>
              <span class="sr-only">访问说明</span>
              <textarea
                v-model="form.accessInstructions"
                rows="7"
                maxlength="4000"
                placeholder="例如服务路径、预置目录及练习前准备。不要填写真实密钥或密码。"
              />
            </label>
            <p class="hint">启用／停用仅影响模板的可选状态，不执行容器启动或停止。</p>
          </aside>
        </div>
      </fieldset>
    </form>

    <button v-else-if="!busy" type="button" @click="load">重试加载</button>
  </div>
</template>

<style scoped>
.template-editor {
  width: 100%;
  color: var(--ink);
}

.back {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--muted);
  text-decoration: none;
  font-size: 12px;
}

.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  margin: 20px 0 24px;
}

.title-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 750;
}

.status-badge {
  font-size: 11px;
  padding: 3px 8px;
  border-radius: 5px;
  background: var(--soft);
  color: var(--muted);
  font-weight: 550;
}

h2 {
  font-size: 15px;
  font-weight: 650;
  margin: 0 0 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.muted,
small,
.hint {
  color: var(--muted);
  font-size: 12px;
}

.editor-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 370px;
  gap: 24px;
  align-items: start;
}

.panel {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 26px;
}

fieldset {
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
}

label {
  display: block;
  font-size: 13px;
  font-weight: 550;
  margin-bottom: 24px;
}

input:not([type='checkbox']),
textarea,
select {
  display: block;
  box-sizing: border-box;
  background: var(--bg);
  color: var(--ink);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 9px 12px;
  font: inherit;
  font-size: 13px;
  margin-top: 8px;
  width: 100%;
}

input[maxlength='120'] {
  max-width: 420px;
}

textarea {
  resize: vertical;
}

small {
  display: block;
  margin-top: 8px;
  line-height: 1.6;
  font-weight: 400;
}

.required,
.error {
  color: var(--error-ink);
}

[aria-invalid='true'] {
  border-color: var(--error-ink) !important;
}

.section-title {
  padding-top: 26px;
  border-top: 1px solid var(--line);
  margin-top: 26px;
}

.toggle {
  display: flex;
  align-items: center;
  gap: 9px;
  margin: 26px 0 0;
  cursor: pointer;
}

.toggle input {
  accent-color: var(--green);
}

button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--paper);
  color: var(--ink);
  padding: 9px 14px;
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
}

button:disabled {
  opacity: 0.45;
  cursor: default;
}

.primary {
  background: var(--ink);
  color: var(--paper);
  border-color: var(--ink);
}

.port-row {
  display: grid;
  grid-template-columns: 110px 96px 38px;
  gap: 9px;
  align-items: center;
  margin-bottom: 12px;
}

.port-row input,
.port-row select,
.port-row .remove-port-btn {
  margin: 0;
  width: 100%;
  height: 38px;
  padding: 0 10px;
  box-sizing: border-box;
}

.port-row .remove-port-btn {
  padding: 0;
}

.hint {
  line-height: 1.7;
  margin: 0 0 18px;
}

.add-port {
  margin-top: 8px;
}

.notice {
  background: var(--soft);
  padding: 14px 16px;
  border-radius: 6px;
  font-size: 13px;
  margin-bottom: 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.failure {
  color: var(--error-ink);
}

</style>
