<script setup lang="ts">
import { Clock3, Terminal, Flag } from 'lucide-vue-next'
import { useEditorGuard } from '~/composables/useEditorGuard'
import type { RuntimePolicy } from '~/shared/runtime-policy'
import { parseRuntimePolicy } from '~/shared/runtime-policy'
import { apiErrorMessage, apiFetch } from '~/utils/api'
definePageMeta({ layout: 'admin' })
useHead({ title: '运行策略 — AttackBox' })
const { data, pending, error, refresh } = await useLazyFetch<{
  data: RuntimePolicy
}>('/api/admin/runtime-policy', { key: 'admin:runtime-policy' })
const draft = ref<RuntimePolicy | null>(null),
  saved = ref(''),
  busy = ref(false),
  message = ref(''),
  failed = ref(false)
const dirty = computed(() => !!draft.value && JSON.stringify(draft.value) !== saved.value)
function applyPolicy(value: RuntimePolicy) {
  draft.value = { ...value }
  saved.value = JSON.stringify(draft.value)
}
watch(
  data,
  value => {
    if (value && !busy.value && !dirty.value) applyPolicy(value.data)
  },
  { immediate: true }
)
useEditorGuard({
  dirty,
  busy,
  title: '放弃运行策略修改？',
  description: '当前未保存的运行策略将丢失。'
})
const confirmAction = useConfirm()
function reset() {
  if (data.value) applyPolicy(data.value.data)
  message.value = ''
  failed.value = false
}
async function reload() {
  if (busy.value) return
  if (
    dirty.value &&
    !(await confirmAction({
      title: '重新载入运行策略？',
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
  try {
    parseRuntimePolicy(draft.value)
  } catch (e) {
    message.value = (e as Error).message
    failed.value = true
    return
  }
  busy.value = true
  message.value = ''
  try {
    const result = await apiFetch<{ data: RuntimePolicy }>('/api/admin/runtime-policy', {
      method: 'PUT',
      body: draft.value,
      retry: false
    })
    data.value = result
    applyPolicy(result.data)
    failed.value = false
    message.value = '运行策略已保存'
  } catch (e) {
    failed.value = true
    message.value = apiErrorMessage(e, '保存失败，修改已保留')
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <div class="runtime-settings">
    <header>
      <div>
        <span class="eyebrow">SYSTEM SETTINGS</span>
        <h1>运行策略</h1>
        <p>统一设置环境的使用时长与自动回收规则。</p>
      </div>
      <Clock3 :size="24" aria-hidden="true" />
    </header>
    <UiBusyIndicator v-if="pending && !draft" label="运行策略加载" />
    <UiNotice v-else-if="error" variant="error" class="mb-4">
      <div class="flex items-center justify-between gap-3 w-full">
        <span>读取失败。</span>
        <UiButton variant="secondary" size="xs" type="button" @click="refresh()">重试</UiButton>
      </div>
    </UiNotice>
    <form v-else-if="draft" @submit.prevent="save" @input="message = ''">
      <fieldset :disabled="busy">
        <div class="policy-cards">
          <section>
            <h2>
              <Terminal :size="19" />
              沙箱
            </h2>
            <p>CLI 和 GUI 共用同一个沙箱。</p>
            <label for="sandbox-ttl">使用时长</label>
            <div class="number-field">
              <input
                id="sandbox-ttl"
                v-model.number="draft.sandboxTtlMinutes"
                type="number"
                min="1"
                max="1440"
                required
              />
              <span>分钟</span>
            </div>
            <label for="sandbox-idle">空闲回收时间</label>
            <div class="number-field">
              <input
                id="sandbox-idle"
                v-model.number="draft.sandboxIdleMinutes"
                type="number"
                min="1"
                :max="draft.sandboxTtlMinutes"
                required
              />
              <span>分钟</span>
            </div>
            <small>无终端输入或桌面操作达到此时长后回收。</small>
          </section>
          <section>
            <h2>
              <Flag :size="19" />
              靶场
            </h2>
            <p>学习与挑战中的私有靶场使用同一规则。</p>
            <label for="target-ttl">使用时长</label>
            <div class="number-field">
              <input
                id="target-ttl"
                v-model.number="draft.targetTtlMinutes"
                type="number"
                min="1"
                max="1440"
                required
              />
              <span>分钟</span>
            </div>
            <small>到期后由 Worker 回收运行实例，不删除镜像。</small>
          </section>
        </div>
        <div class="policy-note">
          <Clock3 :size="16" />
          <span>
            修改仅影响新启动的环境。运行中的实例保留原到期时间，时长范围为 1–1440 分钟。
          </span>
        </div>
        <footer>
          <UiNotice v-if="message" :variant="failed ? 'error' : 'success'" class="w-full mb-3">
            <div class="flex items-center justify-between gap-3 w-full">
              <span>{{ message }}</span>
              <UiButton v-if="failed" type="button" variant="secondary" size="xs" @click="reload">
                重新载入
              </UiButton>
            </div>
          </UiNotice>
          <UiBadge v-else :variant="dirty ? 'warning' : 'neutral'" dot>{{
            dirty ? '有尚未保存的修改' : '设置已同步'
          }}</UiBadge>
          <div class="flex items-center gap-3">
            <UiButton type="button" variant="secondary" size="sm" :disabled="!dirty" @click="reset">
              取消修改
            </UiButton>
            <UiButton type="submit" variant="primary" size="sm" :loading="busy" :disabled="!dirty">
              保存设置
            </UiButton>
          </div>
        </footer>
      </fieldset>
    </form>
  </div>
</template>
<style scoped>
header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 28px;
}
.eyebrow {
  font-size: 10px;
  letter-spacing: 1.5px;
  color: var(--muted);
}
h1 {
  font-size: 25px;
  margin: 10px 0;
}
header p,
section > p {
  color: var(--muted);
  font-size: 13px;
  margin: 0;
  line-height: 1.8;
}
header > svg {
  color: var(--muted);
}
fieldset {
  padding: 0;
  margin: 0;
  border: 0;
  min-width: 0;
}
.policy-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}
section {
  padding: 28px;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
}
h2 {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 16px;
  margin: 0 0 14px;
}
h2 svg {
  color: var(--muted);
}
label {
  display: block;
  font-size: 12px;
  margin: 26px 0 10px;
}
.number-field {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--muted);
}
input {
  width: 116px;
  box-sizing: border-box;
  padding: 10px 12px;
  border: 1px solid var(--line);
  border-radius: 7px;
  background: var(--bg);
  color: var(--ink);
  font: inherit;
  font-size: 14px;
}
small {
  display: block;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.8;
  margin-top: 14px;
}
.policy-note {
  display: flex;
  align-items: center;
  gap: 9px;
  margin-top: 22px;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.8;
}
footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  border-top: 1px solid var(--line);
  margin-top: 28px;
  padding-top: 22px;
  color: var(--muted);
  font-size: 12px;
}
footer > div {
  display: flex;
  gap: 10px;
}
input:focus-visible,
button:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 3px;
}
</style>
