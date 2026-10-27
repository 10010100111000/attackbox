<script setup lang="ts">
defineProps<{pending:boolean;hasData:boolean}>()
</script>
<template><UiBusyIndicator v-if="pending && !hasData" label="后台内容加载" /></template>
