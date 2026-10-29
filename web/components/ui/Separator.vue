<script setup lang="ts">
import { useSlots } from 'vue'
import { Separator } from 'reka-ui'
import type { ClassValue } from 'clsx'
import { cn } from '~/utils/cn'

interface Props {
  orientation?: 'horizontal' | 'vertical'
  decorative?: boolean
  class?: ClassValue
}

const props = withDefaults(defineProps<Props>(), {
  orientation: 'horizontal',
  decorative: false,
  class: undefined
})

const slots = useSlots()
</script>

<template>
  <Separator
    v-if="orientation === 'vertical'"
    orientation="vertical"
    :decorative="decorative"
    :class="cn('inline-block h-4 w-[1px] bg-line shrink-0 mx-2 align-middle', props.class)"
  />
  <Separator
    v-else-if="slots.default"
    :decorative="decorative"
    :class="cn('flex items-center w-full my-4 text-xs text-muted', props.class)"
  >
    <Separator decorative class="flex-1 border-t border-line" />
    <span class="px-3 shrink-0 select-none">
      <slot />
    </span>
    <Separator decorative class="flex-1 border-t border-line" />
  </Separator>
  <Separator
    v-else
    :decorative="decorative"
    :class="cn('w-full border-t border-line my-4', props.class)"
  />
</template>
