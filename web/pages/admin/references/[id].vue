<script setup lang="ts">
import { ArrowLeft, ExternalLink, Eye, EyeOff, Trash2 } from 'lucide-vue-next'
import type { AdminReference } from '~/shared/references'
definePageMeta({layout:'admin'})
useHead({title:'资料预览 — AttackBox'})
const id=String(useRoute().params.id),confirmAction=useConfirm()
const {data,pending,error,refresh}=await useLazyFetch<{data:AdminReference}>(`/api/admin/references/${id}`)
const item=computed(()=>data.value?.data),busy=ref(false),notice=ref('')
async function perform(intent:'publish'|'unpublish'|'delete'){
  if(!item.value||busy.value)return
  if(intent==='delete'&&!(await confirmAction({title:'删除资料？',description:`“${item.value.title}”将被删除，原有访问链接会失效。Git 源文件保留。`,confirmLabel:'删除资料'})))return
  busy.value=true;notice.value=''
  try{
    await $fetch(`/api/admin/references/${id}`,{method:intent==='delete'?'DELETE':'PUT',body:{intent,lockVersion:item.value.lockVersion},retry:false})
    clearNuxtData(key=>key.startsWith('references:')||key==='admin:references')
    if(intent==='delete')await navigateTo('/admin/references');else await refresh()
  }catch(e:any){notice.value=e?.data?.message||'操作未确认，请重新读取资料'}finally{busy.value=false}
}
</script>
<template>
  <div class="reference-admin">
    <NuxtLink class="back" to="/admin/references"><ArrowLeft :size="14" aria-hidden="true"/>参考资料</NuxtLink>
    <AdminDataLoading :pending="pending" :has-data="!!data"/>
    <UiNotice v-if="error||notice" type="error" class="mb-5">{{notice||'资料不存在或无法读取'}}</UiNotice>
    <template v-if="item">
      <header class="page-heading">
        <div>
          <h1>{{item.title}}</h1>
          <p><UiBadge :variant="item.status==='published'?'success':'neutral'" dot>{{item.status==='published'?'已发布':'未发布'}}</UiBadge> · {{item.sourcePath}} · {{item.sourceCommit.slice(0,12)}}</p>
        </div>
        <div class="actions">
          <UiButton variant="danger" size="sm" :disabled="busy" @click="perform('delete')"><Trash2 :size="14" aria-hidden="true"/>删除</UiButton>
          <UiButton v-if="item.status==='published'" variant="secondary" size="sm" :to="`/references/${item.slug}`" target="_blank"><ExternalLink :size="14" aria-hidden="true"/>查看公开页</UiButton>
          <UiButton variant="primary" size="sm" :loading="busy" @click="perform(item.status==='published'?'unpublish':'publish')">
            <EyeOff v-if="item.status==='published'" :size="14" aria-hidden="true"/><Eye v-else :size="14" aria-hidden="true"/>{{item.status==='published'?'下架':'发布资料'}}
          </UiButton>
        </div>
      </header>
      <p class="muted public-path">公开链接：<code>/references/{{item.slug}}</code></p>
      <article class="panel reference-preview"><p v-if="item.summary" class="reference-summary">{{item.summary}}</p><UiMarkdownView :content="item.bodyMarkdown" :image-urls="item.imageUrls"/></article>
    </template>
  </div>
</template>
<style scoped>
@import '~/assets/css/reference-admin.css';

.public-path {
  margin-bottom: 24px;
  overflow-wrap: anywhere;
}

.public-path code {
  font-size: 11px;
}

.reference-preview {
  padding: 40px 48px;
}

.reference-summary {
  font-size: 14px;
  line-height: 1.9;
  color: var(--muted);
  padding-bottom: 24px;
  border-bottom: 1px solid var(--line);
  margin: 0 0 24px;
}

.reference-preview :deep(.markdown-body) {
  max-width: 960px;
  margin: 0 auto;
  font-size: 16px;
  line-height: 1.9;
}
</style>
