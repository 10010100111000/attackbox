<script setup lang="ts">
import { CheckboxRoot, CheckboxIndicator } from 'reka-ui'
import { Check } from 'lucide-vue-next'
import { cn } from '~/utils/cn'

interface Props {
  modelValue?: boolean
  disabled?: boolean
  required?: boolean
  id?: string
  name?: string
  value?: string
  class?: any
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: false,
  disabled: false,
  required: false,
  id: undefined,
  name: undefined,
  value: undefined,
  class: undefined
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()
</script>

<template>
  <CheckboxRoot
    :model-value="modelValue"
    :disabled="disabled"
    :required="required"
    :id="id"
    :name="name"
    :value="value"
    @update:model-value="val => emit('update:modelValue', Boolean(val))"
    :class="
      cn(
        'peer h-[13px] w-[13px] shrink-0 rounded-[3px] border border-line bg-input',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-1',
        'disabled:cursor-not-allowed disabled:opacity-50',
        'data-[state=checked]:bg-accent data-[state=checked]:border-accent data-[state=checked]:text-accent-foreground',
        'transition-colors inline-flex items-center justify-center p-0 select-none',
        props.class
      )
    "
  >
    <CheckboxIndicator class="flex items-center justify-center text-current w-full h-full">
      <Check class="h-2.5 w-2.5 stroke-[3]" />
    </CheckboxIndicator>
  </CheckboxRoot>
</template>
