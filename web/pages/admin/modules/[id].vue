<script setup lang="ts">
import {defineAsyncComponent,computed} from 'vue'
import type {AdminModuleDetail} from '~/shared/content'
import GitModuleDetail from '~/components/admin/GitModuleDetail.vue'
definePageMeta({layout:'admin',key:route=>route.params.id})
const id=String(useRoute().params.id)
if(id==='new')await navigateTo('/admin/content',{replace:true})
const LegacyModuleEditor=defineAsyncComponent(()=>import('~/components/admin/LegacyModuleEditor.vue'))
const {data,pending,error,refresh}=useLazyFetch<{data:AdminModuleDetail}>(`/api/admin/modules/${id}`,{key:`admin:module:${id}`,immediate:id!=='new'})
const detail=computed(()=>data.value?.data)
useHead({title:computed(()=>`${detail.value?.module.title||'学习模块'} — AttackBox`)})
</script>
<template>
 <div><AdminDataLoading :pending="pending" :has-data="!!detail"/><p v-if="error">模块读取失败。<button @click="refresh()">重新读取</button></p><GitModuleDetail v-else-if="detail?.source" :detail="detail" @refresh="refresh()"/><LegacyModuleEditor v-else-if="detail"/></div>
</template>
