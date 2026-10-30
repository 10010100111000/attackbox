<script setup lang="ts">
import { ArrowLeft } from 'lucide-vue-next'
import {
  emptyAd,
  adPlacements,
  adState,
  type Advertisement,
  type AdInput
} from '~/shared/advertising'
import { apiErrorMessage, apiFetch } from '~/utils/api'
definePageMeta({ layout: 'admin' })
const route = useRoute(),
  id = String(route.params.id),
  isNew = id === 'new'
useHead({ title: isNew ? '新建广告 — AttackBox' : '编辑广告 — AttackBox' })
const form = ref<AdInput>(emptyAd()),
  saved = ref(JSON.stringify(form.value)),
  busy = ref(false),
  message = ref(''),
  failed = ref(false)
const { data, pending, error, refresh } = await useLazyFetch<{ data: Advertisement }>(
  () => `/api/admin/advertisements/${id}`,
  { key: `admin:advertisement:${id}`, immediate: !isNew, watch: false }
)
const dirty = computed(() => JSON.stringify(form.value) !== saved.value)
function apply(ad: AdInput) {
  const { id: _id, ...input } = structuredClone(toRaw(ad)) as Advertisement
  form.value = input
  saved.value = JSON.stringify(form.value)
}
watch(
  data,
  value => {
    if (value && !busy.value && !dirty.value) apply(value.data)
  },
  { immediate: true }
)
useEditorGuard({
  dirty,
  busy,
  title: '放弃广告修改？',
  description: '未保存的素材与展示配置将丢失。'
})
const localTime = (value: string | null) => {
  if (!value) return ''
  const date = new Date(value)
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
function setTime(field: 'startsAt' | 'endsAt', value: string) {
  form.value[field] = value ? new Date(value).toISOString() : null
}
function placement(value: string, checked: boolean) {
  form.value.placements = checked
    ? adPlacements
        .filter(p => p.value === value || form.value.placements.includes(p.value))
        .map(p => p.value)
    : form.value.placements.filter(p => p !== value)
}
async function save() {
  if (busy.value) return
  busy.value = true
  message.value = ''
  failed.value = false
  try {
    const result = await apiFetch<{ data: Advertisement }>(
      isNew ? '/api/admin/advertisements' : `/api/admin/advertisements/${id}`,
      { method: isNew ? 'POST' : 'PUT', body: structuredClone(toRaw(form.value)), retry: false }
    )
    apply(result.data)
    message.value = '广告已保存'
    if (isNew) {
      busy.value = false
      await navigateTo(`/admin/advertisements/${result.data.id}`, { replace: true })
    }
  } catch (e) {
    failed.value = true
    message.value = apiErrorMessage(e, '保存失败，修改已保留')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AdminListPage
    :title="isNew ? '新建广告' : '编辑广告'"
    description="素材与展示规则分开配置，保存后用于后续页面展示。"
  >
    <template #actions
      ><UiButton to="/admin/advertisements" variant="secondary" size="sm"
        ><ArrowLeft :size="14" aria-hidden="true" />广告列表</UiButton
      ></template
    >
    <UiBusyIndicator v-if="pending && !data && !isNew" label="广告加载" />
    <UiNotice v-else-if="error" variant="error"
      >读取失败。<UiButton variant="secondary" size="xs" @click="refresh()"
        >重试</UiButton
      ></UiNotice
    >
    <form v-else @submit.prevent="save">
      <fieldset :disabled="busy">
        <div class="ad-editor-grid">
          <section class="panel ad-fields">
            <h2>素材</h2>
            <div class="field-pair">
              <label>内部名称 *<UiInput v-model="form.name" bare maxlength="120" required /></label
              ><label>赞助方<UiInput v-model="form.sponsor" bare maxlength="80" /></label>
            </div>
            <label>展示标题 *<UiInput v-model="form.title" bare maxlength="120" required /></label>
            <label
              >图片地址<UiInput
                v-model="form.imageUrl"
                bare
                maxlength="1024"
                placeholder="https://cdn.example.com/ad.webp"
            /></label>
            <label
              >宣传文案<UiTextarea v-model="form.description" rows="3" maxlength="300"
            /></label>
            <label
              >跳转链接 *<UiInput
                v-model="form.href"
                bare
                maxlength="1024"
                placeholder="https://example.com/campaign"
                required
            /></label>
            <div class="field-pair">
              <label
                >按钮文字<UiInput
                  v-model="form.cta"
                  bare
                  maxlength="30"
                  placeholder="了解更多" /></label
              ><label
                >轮换权重<UiInput
                  v-model.number="form.weight"
                  bare
                  type="number"
                  min="1"
                  max="100"
                  required
                  class="weight-input"
              /></label>
            </div>
          </section>
          <div class="ad-editor-side">
            <section class="panel delivery-settings">
              <div class="delivery-heading">
                <h2>展示规则</h2>
                <UiBadge :variant="adState(form) === '展示中' ? 'success' : 'neutral'">{{
                  adState(form)
                }}</UiBadge>
              </div>
              <label class="check-field"><UiCheckbox v-model="form.enabled" />启用展示</label>
              <div class="placement-options" role="group" aria-label="展示位置">
                <span>展示位置</span
                ><label v-for="p in adPlacements" :key="p.value" class="check-field"
                  ><UiCheckbox
                    :model-value="form.placements.includes(p.value)"
                    @update:model-value="checked => placement(p.value, checked)"
                  />{{ p.label }}</label
                >
              </div>
              <div class="field-pair">
                <label
                  >开始时间<UiInput
                    :model-value="localTime(form.startsAt)"
                    bare
                    type="datetime-local"
                    @update:model-value="value => setTime('startsAt', value)" /></label
                ><label
                  >结束时间<UiInput
                    :model-value="localTime(form.endsAt)"
                    bare
                    type="datetime-local"
                    @update:model-value="value => setTime('endsAt', value)"
                /></label>
              </div>
              <p class="field-help">时间使用浏览器本地时区，可留空。同一广告位按权重轮换。</p>
            </section>
            <section class="ad-preview">
              <h2>素材预览</h2>
              <div inert>
                <AdvertisingCard
                  :creative="{
                    id,
                    title: form.title || '展示标题',
                    description: form.description,
                    imageUrl: form.imageUrl,
                    href: form.href,
                    cta: form.cta,
                    sponsor: form.sponsor
                  }"
                />
              </div>
            </section>
          </div>
        </div>
      </fieldset>
      <footer class="save-bar">
        <UiNotice v-if="message" :variant="failed ? 'error' : 'success'">{{ message }}</UiNotice
        ><span v-else class="save-hint">{{ dirty ? '有未保存的修改' : '' }}</span
        ><UiButton
          variant="secondary"
          :disabled="busy || !dirty"
          @click="apply(data?.data || emptyAd())"
          >取消修改</UiButton
        ><UiButton type="submit" :loading="busy" :disabled="!dirty">保存广告</UiButton>
      </footer>
    </form>
  </AdminListPage>
</template>

<style scoped>
fieldset {
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
}
.ad-editor-grid {
  display: grid;
  grid-template-columns: minmax(440px, 1.1fr) minmax(400px, 1fr);
  gap: 24px;
}
.ad-fields,
.delivery-settings {
  padding: 24px;
}
h2 {
  margin: 0 0 22px;
  font-size: 15px;
  font-weight: 650;
}
.ad-fields > label,
.field-pair label {
  display: grid;
  gap: 8px;
  font-size: 12px;
  color: var(--muted);
}
.ad-fields > label,
.field-pair {
  margin-top: 18px;
}
.field-pair {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
}
.weight-input {
  width: 112px;
}
.ad-editor-side {
  min-width: 0;
}
.delivery-heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
}
.check-field {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}
.placement-options {
  display: grid;
  gap: 12px;
  margin: 24px 0;
}
.placement-options > span,
.field-help {
  font-size: 12px;
  color: var(--muted);
}
.field-help {
  margin: 14px 0 0;
  line-height: 1.7;
}
.ad-preview {
  margin-top: 26px;
}
.ad-preview h2 {
  margin-bottom: 14px;
}
.save-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 24px;
}
.save-bar > :first-child {
  margin-right: auto;
}
.save-hint {
  color: var(--muted);
  font-size: 12px;
}
</style>
