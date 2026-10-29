<script setup lang="ts">
import { useSlots } from 'vue'
import { cn } from '~/utils/cn'

interface Props {
  orientation?: 'horizontal' | 'vertical'
  class?: any
}

const props = withDefaults(defineProps<Props>(), {
  orientation: 'horizontal',
  class: undefined
})

const slots = useSlots()
</script>

<template>
  <div
    v-if="orientation === 'vertical'"
    role="separator"
    aria-orientation="vertical"
    :class="cn('inline-block h-4 w-[1px] bg-line shrink-0 mx-2 align-middle', props.class)"
  />
  <div
    v-else-if="slots.default"
    :class="cn('flex items-center w-full my-4 text-xs text-muted', props.class)"
  >
    <div class="flex-1 border-t border-line" aria-hidden="true" />
    <span class="px-3 shrink-0 select-none">
      <slot />
    </span>
    <div class="flex-1 border-t border-line" aria-hidden="true" />
  </div>
  <div v-else role="separator" :class="cn('w-full border-t border-line my-4', props.class)" />
</template>
