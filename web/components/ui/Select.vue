<script setup lang="ts">
import { computed } from 'vue'
import {
  SelectRoot,
  SelectTrigger,
  SelectIcon,
  SelectPortal,
  SelectContent,
  SelectViewport,
  SelectItem,
  SelectItemText,
  SelectItemIndicator
} from 'reka-ui'
import { Check, ChevronDown } from 'lucide-vue-next'
import type { ClassValue } from 'clsx'
import { cn } from '~/utils/cn'

defineOptions({ inheritAttrs: false })

export interface SelectOption {
  value: string
  label: string
  disabled?: boolean
}

const props = withDefaults(
  defineProps<{
    modelValue?: string
    options: SelectOption[]
    placeholder?: string
    disabled?: boolean
    id?: string
    name?: string
    size?: 'sm' | 'md'
    class?: ClassValue
  }>(),
  {
    modelValue: '',
    placeholder: '请选择',
    disabled: false,
    id: undefined,
    name: undefined,
    size: 'sm',
    class: undefined
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

// reka-ui 的 SelectItem 不接受空字符串值,用哨兵值承载“全部/未选择”这类选项,
// 对外仍然是调用方熟悉的空字符串。
const EMPTY = '__empty__'
const internal = computed(() => (props.modelValue === '' ? EMPTY : props.modelValue))
const options = computed(() =>
  props.options.map(option => ({
    ...option,
    value: option.value === '' ? EMPTY : option.value
  }))
)

// 选中项文案由自己渲染:reka 的 SelectValue 在 SSR 阶段拿不到条目文本,
// 首屏会先显示占位文案,水合后才变成真实值。
const selectedLabel = computed(
  () => props.options.find(option => option.value === props.modelValue)?.label || props.placeholder
)

function onValueChange(value: unknown) {
  emit('update:modelValue', value === EMPTY ? '' : String(value))
}

const sizeClasses: Record<'sm' | 'md', string> = {
  sm: 'h-9 px-3 text-xs',
  md: 'h-10 px-3.5 text-sm'
}
</script>

<template>
  <SelectRoot
    :model-value="internal"
    :disabled="disabled"
    :name="name"
    @update:model-value="onValueChange"
  >
    <SelectTrigger
      :id="id"
      v-bind="$attrs"
      :class="
        cn(
          'inline-flex items-center justify-between gap-2 rounded-md border border-[var(--select-border)] bg-[var(--select-bg)] text-left text-[var(--select-text)] shadow-[var(--surface-highlight)] transition-colors',
          'hover:border-[var(--select-hover-border)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--focus)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--paper)]',
          'disabled:cursor-not-allowed disabled:opacity-50',
          sizeClasses[size] || sizeClasses.sm,
          props.class
        )
      "
    >
      <span class="truncate">{{ selectedLabel }}</span>
      <SelectIcon as-child>
        <ChevronDown class="h-3.5 w-3.5 shrink-0 opacity-70" aria-hidden="true" />
      </SelectIcon>
    </SelectTrigger>

    <SelectPortal>
      <SelectContent
        position="popper"
        :side-offset="4"
        class="z-50 max-h-72 min-w-[var(--reka-select-trigger-width)] overflow-hidden rounded-lg border border-[var(--popover-border)] bg-[var(--popover-bg)] shadow-[var(--card-shadow)]"
      >
        <SelectViewport class="p-1">
          <SelectItem
            v-for="option in options"
            :key="option.value"
            :value="option.value"
            :disabled="option.disabled"
            class="relative flex cursor-pointer select-none items-center gap-2 rounded-sm px-2.5 py-1.5 pr-7 text-xs text-[var(--ink)] outline-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[highlighted]:bg-[var(--hover)]"
          >
            <SelectItemText>{{ option.label }}</SelectItemText>
            <SelectItemIndicator class="absolute right-2 inline-flex items-center">
              <Check class="h-3.5 w-3.5" aria-hidden="true" />
            </SelectItemIndicator>
          </SelectItem>
        </SelectViewport>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>
