<script setup lang="ts">
import { Plus, ArrowUp, ArrowDown, Trash2 } from 'lucide-vue-next'
import { useEditorGuard } from '~/composables/useEditorGuard'
import { parseLearnHome, type LearnHome, type HomeSlide } from '~/shared/learn-home'
import { apiErrorMessage, apiFetch } from '~/utils/api'

const confirmAction = useConfirm()
definePageMeta({ layout: 'admin' })
useHead({ title: '首页管理 — AttackBox' })
const { data, pending, error, refresh } = await useLazyFetch<{ data: LearnHome }>(
  '/api/admin/learn-home'
)
const draft = ref<LearnHome | null>(null),
  selected = ref(0),
  busy = ref(false),
  message = ref(''),
  failed = ref(false)
const saved = ref('')
const clone = (v: LearnHome) => JSON.parse(JSON.stringify(v)) as LearnHome
const dirty = computed(() => !!draft.value && JSON.stringify(draft.value) !== saved.value)
function applyHome(value: LearnHome) {
  draft.value = clone(value)
  selected.value = Math.min(selected.value, Math.max(0, value.slides.length - 1))
  saved.value = JSON.stringify(draft.value)
}
watch(
  data,
  v => {
    if (v && !busy.value && !dirty.value) applyHome(v.data)
  },
  { immediate: true }
)
const item = computed(() => draft.value?.slides[selected.value])
const { data: modules } = useLazyFetch<{ data: { title: string; slug: string }[] }>('/api/modules')
const { data: routes } = useLazyFetch<{ data: { title: string; id: string }[] }>('/api/routes')
function add() {
  if (!draft.value || draft.value.slides.length >= 10) return
  draft.value.slides.push({
    id: crypto.randomUUID(),
    enabled: true,
    label: '模块推荐',
    title: '',
    description: '',
    cta: '查看模块',
    href: '/learn',
    imageUrl: ''
  })
  selected.value = draft.value.slides.length - 1
}
function replaceItem(value: HomeSlide) {
  if (draft.value && !busy.value) draft.value.slides[selected.value] = value
}
function remove() {
  if (!draft.value) return
  draft.value.slides.splice(selected.value, 1)
  selected.value = Math.max(0, selected.value - 1)
}
function move(delta: number) {
  const a = draft.value?.slides
  if (!a) return
  const n = selected.value + delta
  if (n < 0 || n >= a.length) return
  ;[a[selected.value], a[n]] = [a[n], a[selected.value]]
  selected.value = n
}
function reset() {
  if (data.value) applyHome(data.value.data)
  selected.value = 0
  message.value = ''
}
async function reload() {
  if (busy.value) return
  if (
    dirty.value &&
    !(await confirmAction({
      title: '重新载入首页配置？',
      description: '当前未保存的修改将被替换。',
      confirmLabel: '重新载入',
      cancelLabel: '继续编辑'
    }))
  )
    return
  if (busy.value) return
  busy.value = true
  try {
    await refresh()
    if (!error.value) reset()
  } finally {
    busy.value = false
  }
}
async function save() {
  if (!draft.value || busy.value) return
  busy.value = true
  message.value = ''
  try {
    const body = parseLearnHome(draft.value)
    const result = await apiFetch<{ data: LearnHome }>('/api/admin/learn-home', {
      method: 'PUT',
      body,
      retry: false
    })
    data.value = result
    applyHome(result.data)
    failed.value = false
    message.value = '首页已更新'
  } catch (e) {
    failed.value = true
    message.value = apiErrorMessage(e, '保存失败，修改已保留')
  } finally {
    busy.value = false
  }
}
useEditorGuard({
  dirty,
  busy,
  title: '放弃首页修改？',
  description: '当前未保存的首页配置将丢失。'
})
</script>
<template>
  <AdminListPage
    class="home-manager"
    title="首页管理"
    description="设置轮播图、活动宣传和跳转链接。"
  >
    <template #actions
      ><UiButton variant="secondary" size="sm" to="/learn" target="_blank" rel="noopener"
        >查看学习首页</UiButton
      ></template
    >
    <UiBusyIndicator v-if="pending && !draft" label="首页设置加载" />
    <p v-else-if="error" role="alert">
      读取失败。<UiButton variant="secondary" size="xs" @click="reload">重试</UiButton>
    </p>
    <form v-else-if="draft" @submit.prevent="save">
      <fieldset :disabled="busy">
        <section class="settings">
          <label><UiCheckbox v-model="draft.enabled" />显示推荐区</label
          ><label><UiCheckbox v-model="draft.autoplay" />自动轮播</label
          ><label
            >间隔
            <UiInput
              v-model.number="draft.intervalSeconds"
              bare
              type="number"
              min="5"
              max="30"
              required
              :disabled="!draft.autoplay"
            />
            秒</label
          >
        </section>
        <div class="workspace">
          <aside>
            <div class="list-heading">
              <h2>
                推荐内容 <small>{{ draft.slides.length }}/10</small>
              </h2>
              <UiButton
                variant="secondary"
                size="sm"
                type="button"
                :disabled="draft.slides.length >= 10"
                aria-label="添加推荐"
                @click="add"
                ><Plus :size="18"
              /></UiButton>
            </div>
            <button
              v-for="(s, i) in draft.slides"
              :key="s.id"
              type="button"
              class="slide-row"
              :class="{ selected: selected === i }"
              @click="selected = i"
            >
              <span>{{ String(i + 1).padStart(2, '0') }}</span
              ><strong>{{ s.title || '未命名推荐' }}</strong
              ><small>{{ s.enabled ? '显示' : '隐藏' }}</small>
            </button>
            <p v-if="!draft.slides.length">暂无推荐，点击添加开始。</p>
          </aside>
          <section v-if="item" class="editor">
            <div class="list-heading">
              <h2>编辑推荐</h2>
              <div class="tools">
                <label class="visibility-toggle"
                  ><UiCheckbox v-model="item.enabled" />在首页显示</label
                >
                <UiButton
                  variant="secondary"
                  size="sm"
                  type="button"
                  aria-label="上移"
                  :disabled="selected === 0"
                  @click="move(-1)"
                  ><ArrowUp :size="17" /></UiButton
                ><UiButton
                  variant="secondary"
                  size="sm"
                  type="button"
                  aria-label="下移"
                  :disabled="selected === draft.slides.length - 1"
                  @click="move(1)"
                  ><ArrowDown :size="17" /></UiButton
                ><UiButton
                  variant="secondary"
                  size="sm"
                  type="button"
                  aria-label="删除推荐"
                  @click="remove"
                  ><Trash2 :size="17"
                /></UiButton>
              </div>
            </div>
            <AdminHomeSlideFields :model-value="item" @update:model-value="replaceItem" />
            <datalist id="home-destinations">
              <option v-for="m in modules?.data" :key="m.slug" :value="`/learn/modules/${m.slug}`">
                {{ m.title }}
              </option>
              <option v-for="r in routes?.data" :key="r.id" :value="`/learn/routes/${r.id}`">
                {{ r.title }}
              </option>
            </datalist>
          </section>
        </div>
        <section v-if="item" class="preview">
          <h2>当前项预览 <small>未保存的修改仅在此处展示</small></h2>
          <div inert>
            <LearningHomeCarousel :slides="[item]" />
          </div>
        </section>
      </fieldset>
      <footer>
        <p :role="failed ? 'alert' : 'status'">{{ message || (dirty ? '有未保存的修改' : '') }}</p>
        <UiButton
          variant="secondary"
          size="sm"
          type="button"
          :disabled="busy || !dirty"
          @click="reset"
          >取消修改</UiButton
        ><UiButton variant="secondary" size="sm" type="button" :disabled="busy" @click="reload"
          >重新载入</UiButton
        ><UiButton type="submit" :loading="busy" :disabled="busy || !dirty">保存首页</UiButton>
      </footer>
    </form>
  </AdminListPage>
</template>
<style scoped>
.home-manager {
  color: var(--ink);
  width: 100%;
}

.list-heading,
footer,
.tools,
.settings {
  display: flex;
  align-items: center;
  gap: 16px;
}

h2 {
  font-size: 15px;
  margin: 0;
}

p,
small {
  color: var(--muted);
}

small {
  font-size: 12px;
  font-weight: 400;
}

fieldset {
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
}

.slide-row {
  cursor: pointer;
  border: 1px solid var(--line);
  background: var(--paper);
  color: var(--ink);
  border-radius: 7px;
  padding: 8px 12px;
  display: flex;
  width: 100%;
  gap: 10px;
  text-align: left;
  margin-bottom: 8px;
  align-items: center;
  border-color: transparent;
}

.settings {
  padding: 18px 22px;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 10px;
  margin-bottom: 20px;
}

.settings label {
  display: flex;
  align-items: center;
  gap: 8px;
}

.settings input[type='number'] {
  width: 72px;
}

.workspace {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  border: 1px solid var(--line);
  background: var(--paper);
  border-radius: 10px;
  overflow: hidden;
  min-height: 350px;
}

aside {
  border-right: 1px solid var(--line);
  padding: 20px;
}

.list-heading {
  justify-content: space-between;
  margin-bottom: 20px;
}

.slide-row strong {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.slide-row span {
  font-size: 12px;
  color: var(--muted);
}

.slide-row.selected {
  background: var(--soft);
  border-color: var(--line);
}

.slide-row:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.editor {
  padding: 22px;
}

.tools {
  gap: 6px;
}

.visibility-toggle {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-right: 10px;
  font-size: 12px;
  color: var(--muted);
}

.preview {
  margin-top: 26px;
}

.preview h2 {
  margin-bottom: 14px;
}

footer {
  margin-top: 22px;
}

footer p {
  flex: 1;
}
</style>
