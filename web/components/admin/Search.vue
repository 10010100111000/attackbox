<script setup lang="ts">
import { Search } from 'lucide-vue-next'
withDefaults(
  defineProps<{ modelValue: string; label: string; placeholder: string; maxlength?: number }>(),
  { maxlength: 100 }
)
defineEmits<{ 'update:modelValue': [value: string] }>()
</script>

<template>
  <label class="admin-search">
    <span class="sr-only">{{ label }}</span>
    <Search :size="15" class="search-icon" aria-hidden="true" />
    <UiInput
      bare
      size="sm"
      :model-value="modelValue"
      :placeholder="placeholder"
      :maxlength="maxlength"
      class="pl-8"
      @update:model-value="$emit('update:modelValue', $event)"
    />
  </label>
</template>

<style scoped>
.admin-search {
  position: relative;
  display: block;
  width: 280px;
  flex-shrink: 0;
}
.search-icon {
  position: absolute;
  left: 10px;
  top: 50%;
  transform: translateY(-50%);
  pointer-events: none;
  color: var(--muted);
}
</style>
