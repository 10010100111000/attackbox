<script setup lang="ts">
defineProps<{pending:boolean;hasData:boolean}>()
</script>
<template><div v-if="pending && !hasData" class="admin-data-loading" role="status" aria-live="polite">正在加载数据…</div></template>
<style scoped>.admin-data-loading{padding:24px 0;color:var(--muted);font-size:13px;min-height:64px}</style>
