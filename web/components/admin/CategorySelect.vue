<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useFetch } from '#imports'
import { Check, ChevronDown, Plus, X } from 'lucide-vue-next'
import {
  DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription, DialogClose,
} from 'reka-ui'
import type { ContentCategory } from '~/shared/content'
const props = defineProps<{ modelValue: string | null; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string | null] }>()
const { data, error: loadError, refresh } = await useFetch<{ data: ContentCategory[] }>('/api/categories')
const categories = computed(() => data.value?.data || [])
const pickerOpen = ref(false)
const selectedCategory = computed(() => categories.value.find(category => category.id === props.modelValue)?.name || '未分类')
const dialogOpen = ref(false), categoryName = ref(''), saving = ref(false), formError = ref('')
const nameInput = ref<HTMLInputElement | null>(null)
async function selectCategory(value: unknown) {
  if (typeof value !== 'string') return
  pickerOpen.value = false
  if (value === '__create__') {
    categoryName.value = ''; formError.value = ''
    await nextTick()
    dialogOpen.value = true
  } else {
    emit('update:modelValue', value === '__none__' ? null : value)
    await closePicker()
  }
}
async function closePicker() { pickerOpen.value = false; await nextTick(); document.getElementById('module-category')?.focus() }
async function focusName(event: Event) { event.preventDefault(); await nextTick(); nameInput.value?.focus() }
function restoreFocus(event: Event) { event.preventDefault(); document.getElementById('module-category')?.focus() }
async function createCategory() {
  if (saving.value) return
  const name = categoryName.value.trim()
  if (!name) { formError.value = '请输入分类名称'; return }
  saving.value = true; formError.value = ''
  try {
    const result = await $fetch<{ data: ContentCategory }>('/api/admin/categories', { method: 'POST', body: { name } })
    // Update locally first: a failed refresh must not hide a successfully created category.
    if (!categories.value.some(category => category.id === result.data.id))
      data.value = { data: [...categories.value, result.data] }
    emit('update:modelValue', result.data.id)
    dialogOpen.value = false
  } catch (error: any) {
    formError.value = error?.data?.message || error?.data?.statusMessage || '创建失败，请重试'
    await nextTick(); nameInput.value?.focus()
  } finally { saving.value = false }
}
</script>

<template>
  <div class="category-control">
    <div class="category-trigger-row">
      <button id="module-category" type="button" class="category-trigger" :disabled="disabled" aria-label="内容分类" :aria-expanded="pickerOpen" aria-controls="category-options" @click="pickerOpen = !pickerOpen" @keydown.esc.stop.prevent="closePicker">
        <span>{{ selectedCategory }}</span><ChevronDown :size="14" aria-hidden="true" />
      </button>
      <button type="button" class="category-add" :disabled="disabled" aria-label="新建分类" @click="selectCategory('__create__')"><Plus :size="15" aria-hidden="true" /></button>
    </div>
    <div v-if="pickerOpen" id="category-options" class="category-menu" role="group" aria-label="选择内容分类" @keydown.esc.stop.prevent="closePicker">
      <button type="button" class="category-option" :aria-pressed="!modelValue" @click="selectCategory('__none__')"><span>未分类</span><Check v-if="!modelValue" :size="14" aria-hidden="true" /></button>
      <button v-for="category in categories" :key="category.id" type="button" class="category-option" :aria-pressed="modelValue === category.id" @click="selectCategory(category.id)"><span>{{ category.name }}</span><Check v-if="modelValue === category.id" :size="14" aria-hidden="true" /></button>
    </div>
    <p v-if="loadError" class="category-load-error" role="status">分类加载失败 <button type="button" @click="refresh()">重试</button></p>
    <DialogRoot v-model:open="dialogOpen">
      <DialogPortal>
        <DialogOverlay class="category-overlay" />
        <DialogContent class="category-dialog" @open-auto-focus="focusName" @close-auto-focus="restoreFocus" @escape-key-down="saving && $event.preventDefault()" @pointer-down-outside="saving && $event.preventDefault()">
          <DialogTitle class="category-dialog-title">新建分类</DialogTitle>
          <DialogDescription class="category-dialog-description">分类会保存到公共分类列表，供所有模块使用。</DialogDescription>
          <DialogClose type="button" class="category-close" :disabled="saving" aria-label="关闭新建分类"><X :size="17" /></DialogClose>
          <form @submit.prevent="createCategory">
            <label for="new-category-name" class="category-name-label">分类名称</label>
            <input id="new-category-name" ref="nameInput" v-model="categoryName" class="category-name-input" maxlength="80" placeholder="例如：Web 安全" :disabled="saving" :aria-invalid="!!formError" :aria-describedby="formError ? 'category-error' : undefined" @input="formError = ''" />
            <p v-if="formError" id="category-error" class="category-form-error" role="alert">{{ formError }}</p>
            <div class="category-dialog-actions"><DialogClose type="button" class="category-cancel" :disabled="saving">取消</DialogClose><button class="category-submit" type="submit" :disabled="saving || !categoryName.trim()">{{ saving ? '创建中…' : '创建并选中' }}</button></div>
          </form>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>
  </div>
</template>

<style scoped>
.category-control { width: 256px; max-width: 100%; }
.category-trigger-row { display: flex; gap: 6px; }
.category-add { display: grid; place-items: center; flex-shrink: 0; width: 38px; border: 1px solid var(--line); border-radius: 7px; background: var(--paper); color: var(--muted); cursor: pointer; }
.category-add:hover { background: var(--soft); color: var(--ink); }
.category-add:focus-visible, .category-option:focus-visible { outline: 2px solid var(--green); outline-offset: -2px; }
.category-trigger { display: flex; justify-content: space-between; align-items: center; gap: 12px; width: 100%; min-height: 38px; padding: 8px 12px; border: 1px solid var(--line); border-radius: 7px; background: var(--paper); color: var(--ink); font-size: 13px; text-align: left; cursor: pointer; }
.category-trigger :deep([data-placeholder]), .category-trigger :deep(span) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.category-trigger svg { flex-shrink: 0; color: var(--muted); transition: transform .15s; }
.category-trigger[aria-expanded="true"] svg { transform: rotate(180deg); }
.category-trigger:focus-visible, .category-name-input:focus { outline: 2px solid var(--green); outline-offset: 2px; }
.category-trigger:disabled { opacity: .6; cursor: default; }
.category-menu { display: grid; gap: 2px; max-height: 220px; overflow-y: auto; margin-top: 8px; padding: 5px; border: 1px solid var(--line); border-radius: 7px; background: var(--paper); color: var(--ink); }
.category-option { display: flex; align-items: center; justify-content: space-between; gap: 8px; width: 100%; padding: 8px 10px; border: 0; background: transparent; color: var(--ink); border-radius: 4px; font-size: 13px; text-align: left; cursor: pointer; overflow-wrap: anywhere; }
.category-option:hover, .category-option[aria-pressed='true'] { background: var(--soft); }
.category-load-error, .category-form-error { font-size: 12px; margin: 8px 0 0; color: var(--color-error, #b43e3e); }
.category-load-error button { background: transparent; border: 0; color: var(--green); cursor: pointer; }
.category-overlay { position: fixed; inset: 0; background: #0005; z-index: 110; }
.category-dialog { position: fixed; z-index: 111; left: 50%; top: 50%; transform: translate(-50%, -50%); width: min(400px, calc(100vw - 32px)); padding: 26px; border: 1px solid var(--line); border-radius: 12px; background: var(--paper); color: var(--ink); box-shadow: 0 16px 60px #0003; }
.category-dialog-title { font-size: 17px; font-weight: 650; margin: 0 0 10px; }
.category-dialog-description { font-size: 12px; line-height: 1.7; color: var(--muted); margin: 0 0 24px; max-width: 300px; }
.category-close { position: absolute; top: 20px; right: 18px; padding: 4px; border: 0; background: transparent; color: var(--muted); cursor: pointer; }
.category-name-label { display: block; font-size: 12px; font-weight: 600; margin-bottom: 8px; }
.category-name-input { width: 100%; padding: 9px 12px; border: 1px solid var(--line); border-radius: 6px; background: var(--bg); color: var(--ink); font-size: 13px; }
.category-dialog-actions { display: flex; gap: 10px; justify-content: flex-end; margin-top: 24px; }
.category-cancel, .category-submit { border-radius: 6px; padding: 8px 14px; font-size: 13px; cursor: pointer; }
.category-cancel { border: 1px solid var(--line); background: var(--paper); color: var(--ink); }
.category-submit { border: 1px solid var(--ink); background: var(--ink); color: var(--paper); }
.category-dialog button:disabled { opacity: .5; cursor: default; }
</style>
