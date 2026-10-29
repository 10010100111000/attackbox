<script setup lang="ts">
import { ref, computed, useId } from 'vue'
import { Eye, EyeOff } from 'lucide-vue-next'
import type { ClassValue } from 'clsx'
import { cn } from '~/utils/cn'

defineOptions({
  inheritAttrs: false
})

interface Props {
  modelValue?: string | number
  type?: 'text' | 'email' | 'password' | 'number' | 'search'
  size?: 'sm' | 'md'
  id?: string
  name?: string
  placeholder?: string
  error?: string
  autocomplete?: string
  required?: boolean
  disabled?: boolean
  readonly?: boolean
  class?: ClassValue
  inputClass?: ClassValue
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  type: 'text',
  size: 'md',
  id: undefined,
  name: undefined,
  placeholder: undefined,
  error: undefined,
  autocomplete: undefined,
  required: false,
  disabled: false,
  readonly: false,
  class: undefined,
  inputClass: undefined
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'blur', event: FocusEvent): void
}>()

const showPassword = ref(false)
const capsLockActive = ref(false)

const generatedId = useId()
const inputId = computed(() => props.id || generatedId)
const errorId = computed(() => (props.error ? `${inputId.value}-error` : undefined))

const actualType = computed(() => {
  if (props.type === 'password') {
    return showPassword.value ? 'text' : 'password'
  }
  return props.type
})

function handleKeyEvent(e: KeyboardEvent) {
  if (props.type === 'password' && typeof e.getModifierState === 'function') {
    capsLockActive.value = e.getModifierState('CapsLock')
  }
}

function handleBlur(e: FocusEvent) {
  capsLockActive.value = false
  emit('blur', e)
}

function handleInput(e: Event) {
  const target = e.target as HTMLInputElement
  emit('update:modelValue', target.value)
}

const sizeClasses: Record<'sm' | 'md', string> = {
  sm: 'h-8 px-2.5 text-xs',
  md: 'h-10 px-3 py-2 text-sm'
}
</script>

<template>
  <div :class="cn('w-full', props.class)">
    <div class="relative w-full">
      <input
        v-bind="$attrs"
        :id="inputId"
        :name="name"
        :type="actualType"
        :value="modelValue"
        :placeholder="placeholder"
        :autocomplete="autocomplete"
        :required="required"
        :disabled="disabled"
        :readonly="readonly"
        :aria-invalid="error ? 'true' : undefined"
        :aria-describedby="errorId"
        @input="handleInput"
        @keydown="handleKeyEvent"
        @keyup="handleKeyEvent"
        @blur="handleBlur"
        :class="
          cn(
            'flex w-full rounded-md border border-line bg-input text-foreground placeholder:text-muted/60 transition-colors',
            'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent focus-visible:border-accent',
            'disabled:cursor-not-allowed disabled:opacity-50',
            sizeClasses[props.size] || sizeClasses.md,
            type === 'password' && 'pr-10',
            error && 'border-error focus-visible:ring-error focus-visible:border-error',
            inputClass
          )
        "
      />

      <button
        v-if="type === 'password'"
        type="button"
        :disabled="disabled"
        :aria-label="showPassword ? '隐藏密码' : '显示密码'"
        @click="showPassword = !showPassword"
        class="absolute right-3 top-1/2 -translate-y-1/2 text-muted hover:text-foreground transition-colors p-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent rounded-sm disabled:opacity-50 disabled:cursor-not-allowed"
      >
        <EyeOff v-if="showPassword" class="w-4 h-4" />
        <Eye v-else class="w-4 h-4" />
      </button>
    </div>

    <div v-if="capsLockActive || error" class="mt-1 space-y-1">
      <p v-if="capsLockActive" class="text-xs text-accent flex items-center gap-1">
        大写锁定已开启
      </p>
      <p v-if="error" :id="errorId" class="text-xs text-error" role="alert">
        {{ error }}
      </p>
    </div>
  </div>
</template>
