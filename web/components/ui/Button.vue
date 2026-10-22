<script setup lang="ts">
import { computed } from 'vue'
import { Loader2 } from 'lucide-vue-next'
import { cn } from '~/utils/cn'

interface Props {
  variant?: 'primary' | 'outline' | 'ghost'
  size?: 'sm' | 'md' | 'lg'
  loading?: boolean
  disabled?: boolean
  as?: string | any
  to?: string | any
  type?: 'button' | 'submit' | 'reset'
  class?: any
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'primary',
  size: 'md',
  loading: false,
  disabled: false,
  as: 'button',
  to: undefined,
  type: 'button',
  class: undefined
})

const isLink = computed(() => !!props.to || props.as === 'NuxtLink')

const variantClasses: Record<'primary' | 'outline' | 'ghost', string> = {
  primary: 'bg-accent text-accent-foreground font-medium hover:bg-accent-hover shadow-sm active:opacity-90',
  outline: 'border border-line bg-transparent text-foreground hover:bg-card active:opacity-90',
  ghost: 'bg-transparent text-muted hover:text-foreground hover:bg-card active:opacity-90'
}

const sizeClasses: Record<'sm' | 'md' | 'lg', string> = {
  sm: 'h-8 px-3 text-xs rounded-sm gap-1.5',
  md: 'h-10 px-4 text-sm rounded-md gap-2',
  lg: 'h-11 px-5 text-base rounded-lg gap-2.5'
}

const spinnerSizes: Record<'sm' | 'md' | 'lg', string> = {
  sm: 'w-3.5 h-3.5',
  md: 'w-4 h-4',
  lg: 'w-5 h-5'
}

const buttonClasses = computed(() =>
  cn(
    'inline-flex items-center justify-center font-medium transition-colors select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:opacity-50',
    variantClasses[props.variant] || variantClasses.primary,
    sizeClasses[props.size] || sizeClasses.md,
    props.loading ? 'cursor-wait' : (props.disabled && 'pointer-events-none'),
    (props.disabled || props.loading) && isLink.value && 'opacity-50',
    props.disabled && isLink.value && 'pointer-events-none',
    props.class
  )
)
</script>

<template>
  <NuxtLink
    v-if="isLink"
    :to="disabled || loading ? undefined : to"
    :aria-disabled="disabled || loading ? 'true' : undefined"
    :tabindex="disabled || loading ? -1 : undefined"
    :class="buttonClasses"
  >
    <Loader2
      v-if="loading"
      :class="cn('animate-spin-slow shrink-0', spinnerSizes[size])"
      aria-hidden="true"
    />
    <slot />
  </NuxtLink>

  <component
    :is="as"
    v-else
    :type="as === 'button' ? type : undefined"
    :disabled="disabled || loading"
    :aria-disabled="disabled || loading ? 'true' : undefined"
    :class="buttonClasses"
  >
    <Loader2
      v-if="loading"
      :class="cn('animate-spin-slow shrink-0', spinnerSizes[size])"
      aria-hidden="true"
    />
    <slot />
  </component>
</template>
