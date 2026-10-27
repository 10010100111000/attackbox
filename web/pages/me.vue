<script setup lang="ts">
definePageMeta({layout:'learning'})
const route=useRoute()
const section=computed<'learning'|'challenges'|'profile'>(()=>route.query.tab==='challenges'?'challenges':route.query.tab==='profile'?'profile':'learning')
const titles={learning:'学习记录',challenges:'挑战记录',profile:'账户资料'}
useHead({title:computed(()=>`${titles[section.value]} — AttackBox`)})
</script>
<template>
 <AccountShell :section="section"><AccountLearning v-if="section==='learning'"/><AccountChallenges v-else-if="section==='challenges'"/><AccountProfile v-show="section==='profile'" :active="section==='profile'"/></AccountShell>
</template>
