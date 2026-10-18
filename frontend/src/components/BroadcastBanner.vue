<template>
  <div
    v-if="show"
    class="border-b px-4 py-2 text-xs bg-zinc-900 text-zinc-100 transition-all"
  >
    <div class="max-w-5xl mx-auto flex items-center justify-between gap-4">
      <div class="flex items-center gap-2">
        <span class="px-1.5 py-0.5 rounded font-semibold text-[10px] bg-zinc-700 text-white">公告</span>
        <span class="font-medium">{{ message }}</span>
      </div>
      <button
        @click="$emit('dismiss')"
        class="opacity-70 hover:opacity-100 p-0.5 rounded hover:bg-zinc-800 transition-colors"
        title="关闭"
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"></path>
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup>
defineProps({
  show: Boolean,
  message: String,
})
defineEmits(['dismiss'])
</script>
