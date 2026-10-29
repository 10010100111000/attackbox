<script setup lang="ts">
const confirmAction = useConfirm()
import { ref, computed, onMounted } from 'vue'
import { ArrowLeft, Plus, ArrowUp, ArrowDown, X } from 'lucide-vue-next'
import { useEditorGuard } from '~/composables/useEditorGuard'
import TargetTemplateSelect from '~/components/admin/TargetTemplateSelect.vue'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import type { Challenge } from '~/shared/challenges'

definePageMeta({ layout: 'admin', key: r => r.fullPath })
useHead({ title: '编辑挑战 — AttackBox' })

interface StageDraft {
  id?: string
  title: string
  promptMarkdown: string
  hint: string
  flag: string
}

const id = String(useRoute().params.id)
const isNew = id === 'new'
const ready = ref(false)
const loaded = ref(isNew)
const busy = ref(false)
const notice = ref('')
const preview = ref(false)
const conflict = ref(false)
const version = ref(0)
const status = ref('unpublished')

const form = ref({
  title: '',
  summary: '',
  category: '',
  difficulty: 'beginner',
  bodyMarkdown: '',
  targetTemplateId: null as string | null,
  stages: [] as StageDraft[],
})

const original = ref(JSON.stringify(form.value))
const dirty = computed(() => loaded.value && JSON.stringify(form.value) !== original.value)

useEditorGuard({ dirty, busy, title: '放弃挑战修改？', description: '未保存的修改将丢失。' })
onMounted(() => { ready.value = true })

const {
  data: options,
  error: optionsError,
  refresh: retryOptions,
} = await useLazyFetch<{ data: { id: string; name: string; enabled: boolean }[] }>(
  '/api/admin/target-templates/options',
  { key: 'admin:challenge-target-options' }
)

const request = useRequestFetch()

async function load() {
  try {
    const c = (await request<{ data: Challenge }>(`/api/admin/challenges/${id}`)).data
    form.value = {
      title: c.title,
      summary: c.summary,
      category: c.category,
      difficulty: c.difficulty,
      bodyMarkdown: c.bodyMarkdown,
      targetTemplateId: c.targetTemplateId ?? null,
      stages: c.stages.map(s => ({
        id: s.id,
        title: s.title,
        promptMarkdown: s.promptMarkdown,
        hint: s.hint,
        flag: s.flag || '',
      })),
    }
    version.value = c.lockVersion
    status.value = c.status
    original.value = JSON.stringify(form.value)
    loaded.value = true
    conflict.value = false
    notice.value = ''
    return true
  } catch {
    notice.value = '无法读取挑战，请重试'
    return false
  }
}

const initialLoading = ref(!isNew)
onMounted(async () => {
 if (!isNew) {try {await load()} finally {initialLoading.value=false}}
})

function add() {
  form.value.stages.push({
    title: `阶段 ${form.value.stages.length + 1}`,
    promptMarkdown: '',
    hint: '',
    flag: '',
  })
}

function move(i: number, delta: number) {
  const a = [...form.value.stages]
  ;[a[i], a[i + delta]] = [a[i + delta]!, a[i]!]
  form.value.stages = a
}

async function reload() {
  if (busy.value) return
  if (dirty.value && !(await confirmAction({title:'重新读取挑战？',description:'当前未保存的输入将被替换。',confirmLabel:'重新读取',cancelLabel:'继续编辑'}))) return
  if (busy.value) return
  busy.value = true
  try { await load() } finally { busy.value = false }
}

async function save(intent: 'draft' | 'publish') {
  if (busy.value) return
  busy.value = true
  notice.value = ''
  try {
    const r = await $fetch<{ data: { id: string } }>(
      isNew ? '/api/admin/challenges' : `/api/admin/challenges/${id}`,
      {
        method: isNew ? 'POST' : 'PUT',
        body: { ...form.value, lockVersion: version.value, intent },
        retry: false,
      }
    )
    if (isNew) {
      original.value = JSON.stringify(form.value)
      busy.value = false
      await navigateTo(`/admin/challenges/${r.data.id}`)
    } else {
      if (await load()) {
        notice.value = intent === 'publish' ? '已保存并发布' : '已保存为草稿'
      } else {
        conflict.value = true
        notice.value = '内容已保存，重新读取失败，请重试'
      }
    }
  } catch (e: any) {
    notice.value = e?.data?.message || '保存失败，输入已保留'
    conflict.value = e?.statusCode === 409
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="challenge-editor">
    <AdminDataLoading :pending="initialLoading" :has-data="loaded" />
    <NuxtLink class="back" to="/admin/challenges">
      <ArrowLeft :size="14" aria-hidden="true" />
      <span>挑战列表</span>
    </NuxtLink>

    <header class="editor-header">
      <div class="title-group">
        <h1>{{ isNew ? '新建挑战' : '编辑挑战' }}</h1>
        <UiBadge :variant="status === 'published' ? 'success' : 'neutral'" dot>
          {{ status === 'published' ? '已发布' : '草稿' }}
        </UiBadge>
        <UiBadge v-if="dirty" variant="warning">未保存修改</UiBadge>
      </div>
      <div class="actions">
        <UiButton
          type="button"
          variant="secondary"
          size="sm"
          :disabled="!ready || !loaded || busy || conflict"
          @click="save('draft')"
        >
          保存为草稿
        </UiButton>
        <UiButton
          type="button"
          variant="primary"
          size="sm"
          :loading="busy"
          :disabled="!ready || !loaded || busy || conflict"
          @click="save('publish')"
        >
          保存并发布
        </UiButton>
      </div>
    </header>

    <UiNotice v-if="notice" :variant="conflict ? 'error' : 'info'" class="mb-5">
      <div class="flex items-center justify-between gap-3 w-full">
        <span>{{ notice }}</span>
        <UiButton v-if="conflict || !loaded" type="button" variant="secondary" size="xs" @click="reload">重新读取</UiButton>
      </div>
    </UiNotice>

    <fieldset v-if="loaded" :disabled="busy || !ready">
      <div class="editor-grid">
        <section class="panel">
          <h2>挑战内容</h2>
          <label>
            标题 *
            <input v-model="form.title" maxlength="160" />
          </label>

          <div class="text-field">
            <label for="challenge-summary">简介</label>
            <textarea id="challenge-summary" v-model="form.summary" rows="2" maxlength="500" />
          </div>

          <div class="compact">
            <label>
              分类
              <input v-model="form.category" maxlength="60" placeholder="例如 Web 安全" />
            </label>
            <label>
              难度
              <select v-model="form.difficulty">
                <option value="beginner">入门</option>
                <option value="intermediate">进阶</option>
                <option value="advanced">高级</option>
              </select>
            </label>
          </div>

          <div class="section-head">
            <h2>正文</h2>
            <UiButton variant="secondary" size="xs" type="button" @click="preview = !preview">
              {{ preview ? '编辑' : '预览' }}
            </UiButton>
          </div>

          <MarkdownView v-if="preview" :content="form.bodyMarkdown" />
          <div v-else class="text-field">
            <label for="challenge-body" class="sr-only">挑战正文</label>
            <textarea
              id="challenge-body"
              v-model="form.bodyMarkdown"
              rows="12"
              placeholder="使用 Markdown 说明背景与目标"
            />
          </div>

          <label>
            靶机模板
            <TargetTemplateSelect
              id="challenge-target-template"
              v-model="form.targetTemplateId"
              :options="options?.data || []"
              :disabled="!!optionsError"
            />
          </label>
          <p v-if="optionsError" role="alert">
            模板读取失败，原配置保留。
            <UiButton variant="secondary" size="xs" type="button" @click="retryOptions()">重试</UiButton>
          </p>
          <small>关联后，学习者可在挑战页按需启动独享靶机容器。</small>
        </section>

        <section class="panel stages">
          <div class="section-head">
            <h2>阶段 Flag</h2>
            <span>{{ form.stages.length }} / 30</span>
          </div>
          <p class="hint">
            按顺序通过；Flag 区分大小写，忽略首尾空白。通过记录不会因修改 Flag 自动清除。
          </p>

          <div v-for="(s, i) in form.stages" :key="s.id || i" class="stage-editor">
            <div class="section-head">
              <strong>阶段 {{ i + 1 }}</strong>
              <div class="actions">
                <UiButton
                  variant="secondary"
                  size="xs"
                  type="button"
                  :aria-label="`上移阶段 ${i + 1}`"
                  :disabled="i === 0"
                  @click="move(i, -1)"
                >
                  <ArrowUp :size="14" aria-hidden="true" />
                </UiButton>
                <UiButton
                  variant="secondary"
                  size="xs"
                  type="button"
                  :aria-label="`下移阶段 ${i + 1}`"
                  :disabled="i === form.stages.length - 1"
                  @click="move(i, 1)"
                >
                  <ArrowDown :size="14" aria-hidden="true" />
                </UiButton>
                <UiButton
                  variant="danger"
                  size="xs"
                  type="button"
                  :aria-label="`移除阶段 ${i + 1}`"
                  @click="form.stages.splice(i, 1)"
                >
                  <X :size="14" aria-hidden="true" />
                </UiButton>
              </div>
            </div>

            <label>
              阶段标题
              <input v-model="s.title" maxlength="160" />
            </label>

            <div class="text-field">
              <label :for="`stage-prompt-${i}`">任务说明</label>
              <textarea :id="`stage-prompt-${i}`" v-model="s.promptMarkdown" rows="3" />
            </div>

            <label>
              标准 Flag
              <input
                v-model="s.flag"
                maxlength="2000"
                autocomplete="off"
                placeholder="flag{...}"
              />
            </label>

            <div class="text-field">
              <label :for="`stage-hint-${i}`">提示</label>
              <textarea :id="`stage-hint-${i}`" v-model="s.hint" rows="2" maxlength="4000" />
            </div>
          </div>

          <UiButton type="button" variant="secondary" size="sm" :disabled="form.stages.length >= 30" @click="add">
            <Plus :size="14" aria-hidden="true" />
            <span>添加阶段</span>
          </UiButton>
        </section>
      </div>
    </fieldset>
  </div>
</template>

<style scoped>
.challenge-editor {
  width: 100%;
}

.back {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  color: var(--muted);
  font-size: 12px;
  text-decoration: none;
}

.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
  margin: 20px 0 24px;
}

.title-group,
.actions,
.section-head {
  display: flex;
  align-items: center;
  gap: 12px;
}

h1 {
  font-size: 24px;
  font-weight: 750;
  margin: 0;
}

small,
.hint,
.section-head > span {
  font-size: 12px;
  color: var(--muted);
}

.editor-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(340px, 0.85fr);
  gap: 24px;
  align-items: start;
}

.panel {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 26px;
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
  margin-bottom: 20px;
}

input:not([type='checkbox']),
textarea,
select {
  display: block;
  width: 100%;
  box-sizing: border-box;
  background: var(--bg);
  color: var(--ink);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 10px 12px;
  margin-top: 8px;
  font: inherit;
  font-size: 13px;
}

.compact {
  display: grid;
  grid-template-columns: 1fr 150px;
  gap: 20px;
}

.section-head {
  justify-content: space-between;
  margin-bottom: 18px;
}

.section-head h2 {
  margin: 0;
}

.stage-editor {
  border-top: 1px solid var(--line);
  padding-top: 20px;
  margin-top: 22px;
}

.hint {
  line-height: 1.7;
}

.text-field {
  margin-bottom: 20px;
}

.text-field label {
  margin-bottom: 0;
}

fieldset {
  border: 0;
  margin: 0;
  padding: 0;
  min-width: 0;
}

</style>
