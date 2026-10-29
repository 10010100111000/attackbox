<script setup lang="ts">
import { computed } from 'vue'
import type { ClassValue } from 'clsx'

const props = withDefaults(
  defineProps<{
    modelValue: string | null
    id: string
    options: { id: string; name: string; enabled: boolean }[]
    disabled?: boolean
    class?: ClassValue
  }>(),
  { disabled: false, class: undefined }
)

const emit = defineEmits<{
  'update:modelValue': [value: string | null]
}>()

const items = computed(() => {
  const list = [{ value: '', label: '不关联模板' }]
  if (props.modelValue && !props.options.some(template => template.id === props.modelValue)) {
    list.push({ value: props.modelValue, label: '当前模板(信息未加载)' })
  }
  for (const item of props.options.filter(
    template => template.enabled || template.id === props.modelValue
  )) {
    list.push({
      value: item.id,
      label: item.enabled ? item.name : `${item.name}(已停用)`
    })
  }
  return list
})
</script>

<template>
  <UiSelect
    :id="id"
    :model-value="modelValue || ''"
    :options="items"
    :disabled="disabled"
    :class="['mt-2 w-full max-w-[420px]', props.class]"
    @update:model-value="value => emit('update:modelValue', value || null)"
  />
</template>
