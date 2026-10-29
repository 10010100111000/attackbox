<script setup lang="ts">
import { BookOpen, GitBranch, Settings, Download } from 'lucide-vue-next'
import type { AdminReferenceSummary, ReferencePage, ReferenceScan } from '~/shared/references'
definePageMeta({layout:'admin'})
useHead({title:'参考资料管理 — AttackBox'})
const route=useRoute(), page=computed(()=>Number(route.query.page||1))
const { data,pending,error,refresh }=await useLazyFetch<{data:ReferencePage<AdminReferenceSummary>}>('/api/admin/references',{query:{page},key:'admin:references'})
const settingsOpen=ref(false), importing=ref(false), scan=ref<ReferenceScan|null>(null), busy=ref(''), notice=ref('')
const confirmAction=useConfirm()
const stateLabels={new:'待导入',changed:'有更新',unchanged:'已同步',missing:'源文件已移除'}
async function check() {
  if(busy.value)return
  importing.value=true;busy.value='scan';notice.value=''
  try{scan.value=(await $fetch<{data:ReferenceScan}>('/api/admin/references/scan',{method:'POST',retry:false})).data}
  catch(e:any){notice.value=e?.data?.message||'扫描失败，请检查仓库设置';settingsOpen.value=true}
  finally{busy.value=''}
}
async function load(item:ReferenceScan['items'][number]){
  if(busy.value||!scan.value)return
  if(item.state==='changed'&&!(await confirmAction({title:'导入资料更新？',description:'更新会替换本地内容，并转为未发布。预览确认后可重新发布。',confirmLabel:'导入更新',tone:'neutral'})))return
  busy.value=item.sourcePath;notice.value=''
  try{
    const result=await $fetch<{data:{id:string}}>('/api/admin/references/import',{method:'POST',body:{sourcePath:item.sourcePath,lockVersion:item.lockVersion,commit:scan.value.commit,version:scan.value.version},retry:false})
    clearNuxtData('admin:references');await navigateTo(`/admin/references/${result.data.id}`)
  }catch(e:any){notice.value=e?.data?.message||'导入失败，请重新扫描'}finally{busy.value=''}
}
function configured(){scan.value=null;settingsOpen.value=false}
</script>
<template>
  <div class="reference-admin">
    <header class="page-heading">
      <div>
        <h1>参考资料</h1>
        <p>从 Git 导入，预览后公开发布。</p>
      </div>
      <div class="actions">
        <UiButton variant="secondary" size="sm" as="a" href="/api/admin/references/template" download="参考资料示例.md">下载 MD 示例</UiButton>
        <UiButton variant="secondary" size="sm" :aria-expanded="settingsOpen" @click="settingsOpen=!settingsOpen"><Settings :size="14" aria-hidden="true"/>仓库设置</UiButton>
        <UiButton variant="primary" size="sm" :loading="busy==='scan'" :disabled="!!busy" @click="check"><GitBranch :size="14" aria-hidden="true"/>Git 导入</UiButton>
      </div>
    </header>
    <UiNotice v-if="notice" variant="error" class="mb-5">{{notice}}</UiNotice>
    <AdminRepositorySettings v-if="settingsOpen" endpoint="/api/admin/references/repository" @saved="configured"/>
    <section v-if="importing" class="panel" aria-label="Git 资料导入">
      <div class="page-heading"><h2>仓库资料</h2><UiButton variant="secondary" size="xs" :disabled="!!busy" @click="importing=false">收起</UiButton></div>
      <UiBusyIndicator v-if="busy==='scan'" label="扫描资料仓库"/>
      <template v-else-if="scan"><p class="muted">{{scan.commit.slice(0,12)}} · 一份 Markdown 对应一篇资料，名称取文件名。</p>
        <table v-if="scan.items.length"><thead><tr><th>文件</th><th>状态</th><th>操作</th></tr></thead><tbody><tr v-for="item in scan.items" :key="item.sourcePath"><td>{{item.title}}<small>{{item.sourcePath}}</small></td><td>{{stateLabels[item.state]}}</td><td><NuxtLink class="reference-link" v-if="item.state==='unchanged'&&item.id" :to="`/admin/references/${item.id}`">查看预览</NuxtLink><UiButton v-else-if="item.state!=='missing'" variant="secondary" size="xs" :loading="busy===item.sourcePath" :disabled="!!busy" @click="load(item)"><Download :size="13" aria-hidden="true"/>{{item.state==='new'?'导入预览':'导入更新'}}</UiButton><span v-else class="muted">本地资料保留，按需手动下架</span></td></tr></tbody></table>
        <div v-else class="empty">目录中没有资料 Markdown 文件。</div>
      </template>
    </section>
    <AdminDataLoading :pending="pending" :has-data="!!data"/>
    <UiNotice v-if="error" variant="error" class="mb-5">资料列表读取失败。</UiNotice>
    <section v-if="data?.data.items.length" class="panel"><table><thead><tr><th>资料</th><th>状态</th><th>操作</th></tr></thead><tbody><tr v-for="item in data.data.items" :key="item.id"><td><NuxtLink class="reference-link" :to="`/admin/references/${item.id}`">{{item.title}}</NuxtLink><small>{{item.sourcePath}}</small></td><td><UiBadge :variant="item.status==='published'?'success':'neutral'" dot>{{item.status==='published'?'已发布':'未发布'}}</UiBadge></td><td><NuxtLink class="reference-link" :to="`/admin/references/${item.id}`">预览与发布</NuxtLink></td></tr></tbody></table></section>
    <UiEmptyState v-else-if="!pending&&!error" class="panel" description="从 Git 导入第一篇参考资料。"><template #icon><BookOpen :size="30" :stroke-width="1.3" aria-hidden="true"/></template></UiEmptyState>
    <nav v-if="data && data.data.total>24" class="pages" aria-label="资料管理分页"><NuxtLink v-if="page>1" :to="{query:{page:page-1}}">上一页</NuxtLink><span>{{page}}</span><NuxtLink v-if="page*24<data.data.total" :to="{query:{page:page+1}}">下一页</NuxtLink></nav>
  </div>
</template>

<style scoped src="~/assets/css/reference-admin.css"></style>

