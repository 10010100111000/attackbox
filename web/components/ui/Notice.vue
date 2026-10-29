<script setup lang="ts">
import { computed } from 'vue'
import { Info, CheckCircle2, AlertTriangle, AlertCircle, X } from 'lucide-vue-next'
import { cn } from '~/utils/cn'

interface Props {
  variant?: 'info' | 'success' | 'warning' | 'error'
  title?: string
  dismissible?: boolean
  class?: any
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'info',
  title: undefined,
  dismissible: false,
  class: undefined,
})

const emit = defineEmits<{
  (e: 'dismiss'): void
}>()

const iconMap = {
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  error: AlertCircle,
}

const role = computed(() => (props.variant === 'error' || props.variant === 'warning' ? 'alert' : 'status'))

const variantClasses: Record<NonNullable<Props['variant']>, string> = {
  info: 'bg-[var(--soft)] text-[var(--ink)] border-[var(--line)]',
  success: 'bg-[var(--success-bg)] text-[var(--success-ink)] border-transparent',
  warning: 'bg-[rgba(235,160,50,0.14)] text-[#e6a84f] border-transparent',
  error: 'bg-[var(--error-bg)] text-[var(--error-ink)] border-transparent',
}
</script>

<template>
  <div
    :role="role"
    :class="
      cn(
        'flex items-start gap-3 p-3.5 rounded-lg border text-xs leading-relaxed [overflow-wrap:anywhere]',
        variantClasses[variant],
        props.class
      )
    "
  >
    <component
      :is="iconMap[variant]"
      :size="15"
      class="flex-shrink-0 mt-0.5"
      aria-hidden="true"
    />
    <div class="flex-1 min-w-0">
      <strong v-if="title" class="block font-semibold mb-1">{{ title }}</strong>
      <div class="notice-content">
        <slot />
      </div>
    </div>
    <button
      v-if="dismissible"
      type="button"
      aria-label="关闭提示"
      @click="emit('dismiss')"
      class="flex-shrink-0 p-0.5 -mr-1 -mt-1 opacity-60 hover:opacity-100 transition-opacity cursor-pointer rounded"
    >
      <X :size="14" aria-hidden="true" />
    </button>
  </div>
</template>
