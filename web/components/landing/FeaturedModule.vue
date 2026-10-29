<script setup lang="ts">
import ModuleCard from './ModuleCard.vue'
import type { PublicModule } from '~/shared/module-catalog'

defineProps<{ module: PublicModule }>()
</script>

<template>
  <ModuleCard :module="module" featured tone="#d8a57b" />
</template>
