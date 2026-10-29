<script setup lang="ts" generic="T extends string | number = string">
import { ref, computed, useId, useAttrs, mergeProps, h, withDirectives, vModelText } from 'vue'
import { Eye, EyeOff } from 'lucide-vue-next'
import type { ClassValue } from 'clsx'
import { cn } from '~/utils/cn'

defineOptions({ inheritAttrs: false })

interface Props {
  modelValue?: T
  modelModifiers?: { number?: boolean; trim?: boolean; lazy?: boolean }
  type?:
    | 'text'
    | 'email'
    | 'password'
    | 'number'
    | 'search'
    | 'url'
    | 'tel'
    | 'date'
    | 'datetime-local'
    | 'time'
  size?: 'sm' | 'md'
  /** A single native input root, for inline controls and existing label/flex layouts. */
  bare?: boolean
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
  type: 'text',
  size: 'md',
  bare: false,
  modelValue: undefined,
  modelModifiers: () => ({}),
  id: undefined,
  name: undefined,
  placeholder: undefined,
  error: undefined,
  autocomplete: undefined,
  class: undefined,
  inputClass: undefined
})
const emit = defineEmits<{
  'update:modelValue': [value: T]
  blur: [event: FocusEvent]
}>()
const attrs = useAttrs()
const el = ref<HTMLInputElement | null>(null)
defineExpose({ el, focus: () => el.value?.focus() })

const showPassword = ref(false)
const capsLockActive = ref(false)
const generatedId = useId()
const inputId = computed(() => props.id || generatedId)
const errorId = computed(() => (!props.bare && props.error ? `${inputId.value}-error` : undefined))
const actualType = computed(() =>
  !props.bare && props.type === 'password' && showPassword.value ? 'text' : props.type
)
const sizeClasses = {
  sm: 'h-8 px-2.5 text-xs',
  md: 'h-10 px-3 py-2 text-sm'
}

function handleKeyEvent(event: KeyboardEvent) {
  if (!props.bare && props.type === 'password') {
    capsLockActive.value = event.getModifierState?.('CapsLock') ?? false
  }
}
function handleBlur(event: FocusEvent) {
  capsLockActive.value = false
  emit('blur', event)
}

// Both presentations share the same control contract. Attributes and native
// listeners belong to the input; only the enhanced presentation owns a wrapper.
function inputBindings() {
  return mergeProps(attrs, {
    id: inputId.value,
    name: props.name,
    type: actualType.value,
    placeholder: props.placeholder,
    autocomplete: props.autocomplete,
    required: props.required,
    disabled: props.disabled,
    readonly: props.readonly,
    'aria-invalid': props.error ? 'true' : attrs['aria-invalid'],
    'aria-describedby':
      [attrs['aria-describedby'], errorId.value].filter(Boolean).join(' ') || undefined,
    class: cn(
      'flex w-full min-w-0 rounded-md border border-line bg-input text-foreground placeholder:text-muted/60 transition-colors',
      'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent focus-visible:border-accent',
      'disabled:cursor-not-allowed disabled:opacity-50',
      sizeClasses[props.size],
      !props.bare && props.type === 'password' && 'pr-10',
      (props.error || attrs['aria-invalid'] === true || attrs['aria-invalid'] === 'true') &&
        'border-error focus-visible:ring-error focus-visible:border-error',
      props.inputClass,
      props.bare && props.class
    ),
    onKeydown: handleKeyEvent,
    onKeyup: handleKeyEvent,
    onBlur: handleBlur
  })
}

// Reuse Vue's native input directive for IME, .number/.trim/.lazy and DOM
// synchronization. Both layouts render this same control, with no extra element.
const NativeInput = () =>
  withDirectives(
    h('input', {
      ...inputBindings(),
      ref: el,
      'onUpdate:modelValue': (value: string | number) => emit('update:modelValue', value as T)
    }),
    [[vModelText, props.modelValue ?? '', undefined, props.modelModifiers]]
  )
</script>

<template>
  <NativeInput v-if="bare" />
  <div v-else :class="cn('w-full', props.class)">
    <div class="relative w-full">
      <NativeInput />
      <button
        v-if="type === 'password'"
        type="button"
        :disabled="disabled"
        :aria-label="showPassword ? '隐藏密码' : '显示密码'"
        class="absolute right-3 top-1/2 -translate-y-1/2 text-muted hover:text-foreground transition-colors p-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent rounded-sm disabled:opacity-50 disabled:cursor-not-allowed"
        @click="showPassword = !showPassword"
      >
        <EyeOff v-if="showPassword" class="w-4 h-4" />
        <Eye v-else class="w-4 h-4" />
      </button>
    </div>
    <div v-if="capsLockActive || error" class="mt-1 space-y-1">
      <p v-if="capsLockActive" class="text-xs text-accent flex items-center gap-1">
        大写锁定已开启
      </p>
      <p v-if="error" :id="errorId" class="text-xs text-error" role="alert">{{ error }}</p>
    </div>
  </div>
</template>
