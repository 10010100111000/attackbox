<script setup lang="ts">
import { ref } from 'vue'
import type { ClassValue } from 'clsx'
import { cn } from '~/utils/cn'

defineOptions({ inheritAttrs: false })

interface Props {
  modelValue?: string
  rows?: number | string
  placeholder?: string
  disabled?: boolean
  readonly?: boolean
  required?: boolean
  id?: string
  name?: string
  class?: ClassValue
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  rows: undefined,
  placeholder: undefined,
  disabled: false,
  readonly: false,
  required: false,
  id: undefined,
  name: undefined,
  class: undefined
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const el = ref<HTMLTextAreaElement | null>(null)

defineExpose({
  el,
  focus: () => el.value?.focus()
})

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
}
</script>

<template>
  <textarea
    ref="el"
    v-bind="$attrs"
    :id="id"
    :name="name"
    :rows="rows"
    :value="modelValue"
    :placeholder="placeholder"
    :disabled="disabled"
    :readonly="readonly"
    :required="required"
    :class="
      cn(
        'w-full rounded-md border border-line bg-input text-foreground placeholder:text-muted/60 transition-colors',
        'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent focus-visible:border-accent',
        'disabled:cursor-not-allowed disabled:opacity-50',
        props.class
      )
    "
    @input="onInput"
  />
</template>
