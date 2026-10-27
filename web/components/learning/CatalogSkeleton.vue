<script setup lang="ts">
defineProps<{label:string}>()
</script>
<template>
 <div class="catalog-skeleton" role="status" :aria-label="label" aria-busy="true"><div v-for="i in 3" :key="i" class="skeleton-card" aria-hidden="true"><div class="skeleton-cover"><UiBusyIndicator v-if="i===1" inline aria-hidden="true"/></div><span/><span/><span/></div></div>
</template>
<style scoped>
.catalog-skeleton{display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:20px}.skeleton-card{border:1px solid var(--line);border-radius:11px;background:var(--paper);overflow:hidden;min-height:290px}.skeleton-cover{display:grid;place-items:center;font-size:24px;height:108px;background:var(--soft)}span{display:block;height:12px;margin:24px 22px;background:var(--soft);border-radius:6px}span:last-child{width:45%}@media(max-width:1000px){.catalog-skeleton{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:640px){.catalog-skeleton{grid-template-columns:1fr}.skeleton-card:not(:first-child){display:none}}
</style>
