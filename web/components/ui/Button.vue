<script setup lang="ts">
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import type { ClassValue } from 'clsx'
import { cn } from '~/utils/cn'

interface Props {
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger'
  size?: 'xs' | 'sm' | 'md' | 'lg'
  loading?: boolean
  disabled?: boolean
  as?: 'button' | 'a'
  to?: RouteLocationRaw
  href?: string
  type?: 'button' | 'submit' | 'reset'
  class?: ClassValue
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'primary',
  size: 'md',
  loading: false,
  disabled: false,
  as: 'button',
  to: undefined,
  href: undefined,
  type: 'button',
  class: undefined
})

const blocked = computed(() => props.disabled || props.loading)

function guardActivation(event: MouseEvent) {
  if (blocked.value) {
    event.preventDefault()
    event.stopImmediatePropagation()
  }
}

const variantClasses: Record<'primary' | 'secondary' | 'outline' | 'ghost' | 'danger', string> = {
  primary:
    'bg-[var(--btn-primary-bg)] text-[var(--btn-primary-fg)] font-semibold hover:bg-[var(--btn-primary-hover)] shadow-[var(--btn-primary-shadow)] active:opacity-90 transition-all duration-150',
  secondary:
    'border border-[var(--line)] bg-[var(--paper)] text-[var(--ink)] hover:bg-[var(--soft)] hover:text-[var(--ink)] active:opacity-90 font-medium',
  outline: 'border border-line bg-transparent text-foreground hover:bg-card active:opacity-90',
  ghost: 'bg-transparent text-muted hover:text-foreground hover:bg-card active:opacity-90',
  danger:
    'border border-transparent bg-transparent text-[var(--error-ink)] hover:bg-[var(--error-bg)] active:opacity-90 font-medium'
}

const sizeClasses: Record<'xs' | 'sm' | 'md' | 'lg', string> = {
  xs: 'h-7 px-2.5 text-xs rounded-sm gap-1',
  sm: 'h-8 px-3 text-xs rounded-sm gap-1.5',
  md: 'h-10 px-4 text-sm rounded-md gap-2',
  lg: 'h-11 px-5 text-base rounded-lg gap-2.5'
}

const buttonClasses = computed(() =>
  cn(
    'inline-flex items-center justify-center font-medium transition-colors select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:opacity-50',
    variantClasses[props.variant] || variantClasses.primary,
    sizeClasses[props.size] || sizeClasses.md,
    blocked.value && 'opacity-50',
    props.loading ? 'cursor-wait' : props.disabled ? 'cursor-not-allowed' : 'cursor-pointer',
    props.class
  )
)
</script>

<template>
  <NuxtLink
    v-if="to"
    :to="blocked ? undefined : to"
    :aria-disabled="blocked ? 'true' : undefined"
    :aria-busy="loading"
    :tabindex="blocked ? -1 : undefined"
    :class="buttonClasses"
    @click.capture="guardActivation"
  >
    <UiBusyLabel :busy="loading"><slot /></UiBusyLabel>
  </NuxtLink>

  <a
    v-else-if="as === 'a' || href"
    :href="blocked ? undefined : href"
    :aria-disabled="blocked ? 'true' : undefined"
    :aria-busy="loading"
    :tabindex="blocked ? -1 : undefined"
    :class="buttonClasses"
    @click.capture="guardActivation"
  >
    <UiBusyLabel :busy="loading"><slot /></UiBusyLabel>
  </a>

  <button
    v-else
    :type="type"
    :disabled="blocked"
    :aria-disabled="blocked ? 'true' : undefined"
    :aria-busy="loading"
    :class="buttonClasses"
  >
    <UiBusyLabel :busy="loading"><slot /></UiBusyLabel>
  </button>
</template>
