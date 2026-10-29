<script setup lang="ts">
import type { ClassValue } from 'clsx'
import { cn } from '~/utils/cn'

interface Props {
  variant?: 'neutral' | 'success' | 'warning' | 'error' | 'brand'
  dot?: boolean
  class?: ClassValue
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'neutral',
  dot: false,
  class: undefined
})

const variantClasses: Record<'neutral' | 'success' | 'warning' | 'error' | 'brand', string> = {
  neutral: 'bg-[var(--soft)] text-[var(--muted)] border-transparent',
  success: 'bg-[var(--success-bg)] text-[var(--success-ink)] border-transparent',
  warning: 'bg-[rgba(235,160,50,0.14)] text-[#e6a84f] border-transparent',
  error: 'bg-[var(--error-bg)] text-[var(--error-ink)] border-transparent',
  brand: 'bg-[var(--brand-soft)] text-[var(--brand)] border-[var(--brand-border)]'
}

const dotColors: Record<'neutral' | 'success' | 'warning' | 'error' | 'brand', string> = {
  neutral: 'bg-current opacity-60',
  success: 'bg-[var(--success-ink)]',
  warning: 'bg-[#e6a84f]',
  error: 'bg-[var(--error-ink)]',
  brand: 'bg-[var(--brand)]'
}
</script>

<template>
  <span
    :class="
      cn(
        'inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[11px] font-medium border leading-none whitespace-nowrap',
        variantClasses[variant] || variantClasses.neutral,
        props.class
      )
    "
  >
    <i
      v-if="dot"
      :class="cn('w-1.5 h-1.5 rounded-full flex-shrink-0', dotColors[variant] || 'bg-current')"
      aria-hidden="true"
    />
    <slot />
  </span>
</template>
