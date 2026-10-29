<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import type {AdminModuleDetail} from '~/shared/content'
import GitModuleDetail from '~/components/admin/GitModuleDetail.vue'
definePageMeta({layout:'admin',key:route=>route.params.id})
const id=String(useRoute().params.id)
if(id==='new')await navigateTo('/admin/content',{replace:true})
const {data,pending,error,refresh}=useLazyFetch<{data:AdminModuleDetail}>(`/api/admin/modules/${id}`,{key:`admin:module:${id}`,immediate:id!=='new'})
const detail=ref<AdminModuleDetail|null>(data.value?.data||null)
watch(data,value=>{if(value)detail.value=value.data})
async function reloadDetail(): Promise<AdminModuleDetail> {
 await refresh()
 if(error.value||!data.value)throw Error('模块读取失败')
 return data.value.data
}
useHead({title:computed(()=>`${detail.value?.module.title||'学习模块'} — AttackBox`)})
</script>
<template>
 <div><AdminDataLoading :pending="pending" :has-data="!!detail"/><UiNotice v-if="error&&!detail" variant="error">模块读取失败。<UiButton variant="secondary" size="xs" @click="refresh()">重新读取</UiButton></UiNotice><GitModuleDetail v-else-if="detail" :detail="detail" :reload="reloadDetail"/></div>
</template>
