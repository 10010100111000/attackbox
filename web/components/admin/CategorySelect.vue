<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useFetch } from '#imports'
import { Check, ChevronDown, Plus, X } from 'lucide-vue-next'
import {
  SelectRoot, SelectTrigger, SelectValue, SelectPortal, SelectContent, SelectViewport,
  SelectItem, SelectItemText, SelectItemIndicator, SelectSeparator,
  DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription, DialogClose,
} from 'reka-ui'
import type { ContentCategory } from '~/shared/content'
const props = defineProps<{ modelValue: string | null; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string | null] }>()
const { data, error: loadError, refresh } = await useFetch<{ data: ContentCategory[] }>('/api/categories')
const categories = computed(() => data.value?.data || [])
const dialogOpen = ref(false), categoryName = ref(''), saving = ref(false), formError = ref('')
const nameInput = ref<HTMLInputElement | null>(null)
async function selectCategory(value: unknown) {
  if (typeof value !== 'string') return
  if (value === '__create__') {
    categoryName.value = ''; formError.value = ''
    await nextTick()
    dialogOpen.value = true
  } else emit('update:modelValue', value === '__none__' ? null : value)
}
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
    <SelectRoot :model-value="modelValue || '__none__'" :disabled="disabled" @update:model-value="selectCategory">
      <SelectTrigger id="module-category" class="category-trigger" aria-label="内容分类">
        <SelectValue placeholder="选择分类" /><ChevronDown :size="14" aria-hidden="true" />
      </SelectTrigger>
      <SelectPortal>
        <SelectContent class="category-menu" position="popper" :side-offset="6" :collision-padding="12">
          <SelectViewport class="category-viewport">
            <SelectItem value="__none__" class="category-option"><SelectItemText>未分类</SelectItemText><SelectItemIndicator><Check :size="14" /></SelectItemIndicator></SelectItem>
            <SelectItem v-for="category in categories" :key="category.id" :value="category.id" class="category-option"><SelectItemText>{{ category.name }}</SelectItemText><SelectItemIndicator><Check :size="14" /></SelectItemIndicator></SelectItem>
            <SelectSeparator class="category-separator" />
            <SelectItem value="__create__" class="category-option category-create-option"><Plus :size="14" aria-hidden="true" /><SelectItemText>新建分类…</SelectItemText></SelectItem>
          </SelectViewport>
        </SelectContent>
      </SelectPortal>
    </SelectRoot>
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
.category-control { width: 220px; max-width: 100%; }
.category-trigger { display: flex; justify-content: space-between; align-items: center; gap: 12px; width: 100%; min-height: 38px; padding: 8px 12px; border: 1px solid var(--line); border-radius: 7px; background: var(--paper); color: var(--ink); font-size: 13px; text-align: left; cursor: pointer; }
.category-trigger :deep([data-placeholder]), .category-trigger :deep(span) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.category-trigger svg { flex-shrink: 0; color: var(--muted); }
.category-trigger:focus-visible, .category-name-input:focus { outline: 2px solid var(--green); outline-offset: 2px; }
.category-trigger:disabled { opacity: .6; cursor: default; }
.category-menu { z-index: 100; min-width: var(--reka-select-trigger-width); max-width: min(360px, calc(100vw - 24px)); padding: 5px; border: 1px solid var(--line); border-radius: 9px; background: var(--paper); color: var(--ink); box-shadow: 0 8px 28px #0002; }
.category-viewport { max-height: 300px; }
.category-option { position: relative; display: flex; align-items: center; gap: 8px; justify-content: space-between; padding: 9px 10px; border-radius: 5px; font-size: 13px; cursor: pointer; outline: none; overflow-wrap: anywhere; }
.category-option[data-highlighted] { background: var(--soft); }
.category-create-option { justify-content: flex-start; color: var(--green); }
.category-separator { height: 1px; background: var(--line); margin: 5px; }
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
