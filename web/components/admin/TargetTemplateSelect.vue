<script setup lang="ts">
defineProps<{
  modelValue: string | null
  id: string
  options: { id: string; name: string; enabled: boolean }[]
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string | null]
}>()
</script>

<template>
  <select
    :id="id"
    :value="modelValue || ''"
    :disabled="disabled"
    class="target-select"
    @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value || null)"
  >
    <option value="">不关联模板</option>
    <option
      v-if="modelValue && !options.some(t => t.id === modelValue)"
      :value="modelValue"
    >
      当前模板（信息未加载）
    </option>
    <option
      v-for="item in options.filter(t => t.enabled || t.id === modelValue)"
      :key="item.id"
      :value="item.id"
    >
      {{ item.name }}{{ item.enabled ? '' : '（已停用）' }}
    </option>
  </select>
</template>

<style scoped>
.target-select {
  display: block;
  width: 100%;
  max-width: 420px;
  box-sizing: border-box;
  background: var(--paper);
  color: var(--ink);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 9px 12px;
  margin-top: 8px;
  font-size: 13px;
}

.target-select:disabled {
  opacity: 0.6;
}
</style>
