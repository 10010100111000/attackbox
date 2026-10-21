<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-50 bg-zinc-950/40 backdrop-blur-[1px] flex items-center justify-center p-4"
        @click.self="$emit('update:modelValue', false)"
      >
        <div class="w-full max-w-md shadcn-card p-5 shadow-xl">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-sm font-semibold text-zinc-900">{{ title }}</h3>
            <button
              @click="$emit('update:modelValue', false)"
              class="text-zinc-400 hover:text-zinc-700 p-1 rounded hover:bg-zinc-100 transition-colors"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"></path>
              </svg>
            </button>
          </div>
          <slot></slot>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
defineProps({
  modelValue: Boolean,
  title: String,
})
defineEmits(['update:modelValue'])
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
