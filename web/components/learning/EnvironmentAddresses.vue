<script setup lang="ts">
import { Copy, Check } from 'lucide-vue-next'
import type { TargetInstance } from '~/composables/useTargetSession'
const props = defineProps<{ instance?: TargetInstance['instance'] }>()
function host(address: string) {
  try {
    return new URL(address.includes('://') ? address : `tcp://${address}`).hostname.replace(
      /^\[|\]$/g,
      ''
    )
  } catch {
    return address
  }
}
const addresses = computed(() => {
  const data = props.instance
  if (!data) return []
  const target = host(data.ip || data.entry_address || '')
  return [
    ...(target ? [{ label: '靶机 IP', address: target }] : []),
    ...(data.attachment === 'connected' && data.attacker_address
      ? [{ label: '回连 IP', address: data.attacker_address }]
      : [])
  ]
})
const copied = ref(''),
  error = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
async function copy(address: string) {
  try {
    await navigator.clipboard.writeText(address)
    error.value = ''
    copied.value = address
    clearTimeout(timer)
    timer = setTimeout(() => (copied.value = ''), 2000)
  } catch {
    error.value = '复制失败，请手动复制 IP'
  }
}
onBeforeUnmount(() => clearTimeout(timer))
</script>
<template>
  <div v-if="addresses.length" class="environment-addresses">
    <div v-for="item in addresses" :key="item.label" class="address">
      <span>{{ item.label }}</span
      ><code>{{ item.address }}</code>
      <UiButton
        variant="ghost"
        size="xs"
        :aria-label="`复制${item.label}`"
        @click="copy(item.address)"
      >
        <Check v-if="copied === item.address" :size="14" /><Copy v-else :size="14" />
      </UiButton>
    </div>
    <span v-if="error" class="address-error" role="alert">{{ error }}</span>
  </div>
</template>
<style scoped>
.environment-addresses {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 24px;
  margin-top: 12px;
}
.address {
  display: flex;
  align-items: center;
  gap: 8px;
}
span {
  font-size: 12px;
  color: var(--muted);
}
.address-error {
  color: var(--error-ink);
}
code {
  font-size: 14px;
  font-family: ui-monospace, monospace;
  user-select: all;
}
</style>
