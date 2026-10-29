<script setup lang="ts">
import { cn } from '~/utils/cn'

interface Props {
  title?: string
  description?: string
  class?: any
}

defineProps<Props>()
</script>

<template>
  <div
    :class="
      cn(
        'flex flex-col items-center justify-center text-center p-12 sm:p-16 border border-line rounded-xl bg-paper text-muted',
        $props.class
      )
    "
  >
    <div v-if="$slots.icon" class="mb-4 text-muted/70 flex justify-center">
      <slot name="icon" />
    </div>
    <h3 v-if="title" class="text-sm font-semibold text-ink mb-1">
      {{ title }}
    </h3>
    <p v-if="description" class="text-xs text-muted max-w-sm leading-relaxed mb-4">
      {{ description }}
    </p>
    <div v-if="$slots.default" class="text-xs text-muted max-w-sm leading-relaxed">
      <slot />
    </div>
    <div v-if="$slots.action" class="mt-5 flex items-center gap-3">
      <slot name="action" />
    </div>
  </div>
</template>
