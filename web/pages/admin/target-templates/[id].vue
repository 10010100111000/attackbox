<script setup lang="ts">
definePageMeta({layout:'admin'})
await navigateTo(`/admin/target-templates/compose/${String(useRoute().params.id)}`,{replace:true})
</script>
<template><div/></template>
