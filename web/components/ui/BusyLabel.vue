<script setup lang="ts">
defineProps<{busy:boolean}>()
</script>
<template>
 <span class="busy-label" :aria-busy="busy"><span class="busy-copy" :class="{concealed:busy}"><slot/></span><UiBusyIndicator v-if="busy" inline aria-hidden="true" class="busy-mark"/></span>
</template>
<style scoped>
.busy-label{display:inline-grid;place-items:center;vertical-align:middle}.busy-copy,.busy-mark{grid-area:1/1}.busy-copy{display:inline-flex;align-items:center;justify-content:center;gap:8px}.busy-copy.concealed{animation:busy-conceal 0s linear .18s both}@keyframes busy-conceal{from{opacity:1}to{opacity:0}}.busy-mark{pointer-events:none;font-size:16px}
</style>
