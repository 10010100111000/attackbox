<script setup lang="ts">
import {computed,ref,watch,nextTick} from 'vue'
import {Check,Pencil,Trash2,Plus,X,Search,ChevronRight} from 'lucide-vue-next'
import {DialogRoot,DialogTrigger,DialogPortal,DialogOverlay,DialogContent,DialogTitle,DialogDescription,DialogClose} from 'reka-ui'
import type {ContentCategory} from '~/shared/content'
type Category=ContentCategory & {moduleCount:number}
const props=defineProps<{modelValue:string|null;disabled?:boolean}>()
const emit=defineEmits<{'update:modelValue':[value:string|null]; 'version-changed':[value:{id:string;previousVersion:number;lockVersion:number}]}>()
const {data,error:loadError,refresh}=await useLazyFetch<{data:Category[]}>('/api/admin/categories',{key:'admin:categories'})
const categories=computed(()=>data.value?.data||[]),open=ref(false),search=ref(''),busy=ref(false),error=ref('')
const mode=ref<'list'|'create'|'rename'|'delete'>('list'),editing=ref<Category|null>(null),name=ref('')
const selected=computed(()=>categories.value.find(c=>c.id===props.modelValue)?.name||(props.modelValue?'分类待确认':'未分类'))
const filtered=computed(()=>categories.value.filter(c=>c.name.toLocaleLowerCase().includes(search.value.trim().toLocaleLowerCase())))
watch(open,value=>{if(value){mode.value='list';search.value='';error.value='';refresh()}})
function choose(id:string|null){if(busy.value)return;emit('update:modelValue',id);open.value=false}
function edit(kind:'create'|'rename'|'delete',category?:Category){mode.value=kind;editing.value=category||null;name.value=category?.name||'';error.value='';nextTick(()=>document.getElementById('category-name')?.focus())}
async function save(){
 if(busy.value)return;busy.value=true;error.value=''
 try{
  if(mode.value==='delete'){
   const deleted=await $fetch<{data:{affectedModules:{id:string;previousVersion:number;lockVersion:number}[]}}>(`/api/admin/categories/${editing.value!.id}`,{method:'DELETE',body:{currentName:editing.value!.name,moduleCount:editing.value!.moduleCount}})
   for(const module of deleted.data.affectedModules)emit('version-changed',module)
   data.value={data:categories.value.filter(c=>c.id!==editing.value!.id)}
   if(props.modelValue===editing.value!.id)emit('update:modelValue',null)
  }else{
   const creating=mode.value==='create'
   const result=await $fetch<{data:ContentCategory}>(creating?'/api/admin/categories':`/api/admin/categories/${editing.value!.id}`,{method:creating?'POST':'PATCH',body:{name:name.value,currentName:editing.value?.name}})
   const category={...result.data,moduleCount:creating?0:editing.value!.moduleCount}
   data.value={data:creating?[...categories.value,category]:categories.value.map(c=>c.id===category.id?category:c)}
   if(creating)emit('update:modelValue',category.id)
  }
  clearNuxtData(key=>key.startsWith('learning:')||key==='content:categories')
  mode.value='list'
 }catch(e:any){error.value=e?.data?.message||'保存失败，请刷新后重试'}finally{busy.value=false}
}
</script>
<template>
 <DialogRoot v-model:open="open"><DialogTrigger id="module-category" class="category-trigger" :disabled="disabled"><span>{{selected}}</span><ChevronRight :size="15" aria-hidden="true"/></DialogTrigger>
 <DialogPortal><DialogOverlay class="category-overlay"/><DialogContent class="category-drawer" @escape-key-down="busy && $event.preventDefault()" @pointer-down-outside="busy && $event.preventDefault()">
  <header><DialogTitle>内容分类</DialogTitle><DialogClose :disabled="busy" aria-label="关闭分类管理"><X :size="18"/></DialogClose></header>
  <DialogDescription class="category-description">选择当前模块的分类，也可在这里管理分类名称。</DialogDescription>
  <template v-if="mode==='list'">
   <div class="category-tools"><label><Search :size="15"/><input v-model="search" placeholder="搜索分类" aria-label="搜索分类"/></label><button type="button" @click="edit('create')"><Plus :size="15"/>新建</button></div>
   <p v-if="loadError" role="alert">分类加载失败。<button @click="refresh()">重试</button></p>
   <div class="category-list"><button class="category-choice" @click="choose(null)"><span>未分类</span><Check v-if="!modelValue" :size="16"/></button>
    <div v-for="category in filtered" :key="category.id" class="category-row"><button class="category-choice" :aria-pressed="modelValue===category.id" @click="choose(category.id)"><span>{{category.name}}<small>{{category.moduleCount}} 个模块</small></span><Check v-if="modelValue===category.id" :size="16"/></button><button :aria-label="`重命名 ${category.name}`" @click="edit('rename',category)"><Pencil :size="15"/></button><button :aria-label="`删除 ${category.name}`" @click="edit('delete',category)"><Trash2 :size="15"/></button></div>
    <p v-if="search && !filtered.length" class="muted">没有匹配的分类</p>
   </div>
  </template>
  <form v-else @submit.prevent="save"><h3>{{mode==='create'?'新建分类':mode==='rename'?'重命名分类':'删除分类'}}</h3><template v-if="mode!=='delete'"><label for="category-name">分类名称</label><input id="category-name" v-model="name" maxlength="80" :disabled="busy" required/></template><p v-else>确认删除「{{editing?.name}}」？<span v-if="editing?.moduleCount">关联的 {{editing.moduleCount}} 个模块会改为未分类，正文和发布状态保持不变。</span></p><p v-if="error" role="alert">{{error}}</p><footer><button type="button" :disabled="busy" @click="mode='list'">取消</button><button type="submit" :disabled="busy || (mode!=='delete' && !name.trim())" :class="{danger:mode==='delete'}">{{busy?'处理中…':mode==='delete'?'确认删除':'保存'}}</button></footer></form>
 </DialogContent></DialogPortal></DialogRoot>
</template>
<style scoped>
.category-trigger{display:flex;align-items:center;justify-content:space-between;gap:16px;width:256px;height:38px;padding:0 12px;border:1px solid var(--line);border-radius:7px;background:var(--paper);color:var(--ink);font-size:13px;cursor:pointer}.category-trigger span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.category-trigger svg{flex-shrink:0}.category-trigger:disabled{opacity:.5;cursor:default}.category-overlay{position:fixed;inset:0;background:#0005;z-index:110}.category-drawer{position:fixed;inset:0 0 0 auto;width:440px;display:flex;flex-direction:column;padding:28px;background:var(--paper);color:var(--ink);border-left:1px solid var(--line);box-shadow:-12px 0 48px #0002;z-index:111;overflow-y:auto}.category-drawer header{display:flex;align-items:center;justify-content:space-between;font-size:18px;font-weight:600}.category-drawer :deep(button){cursor:pointer}.category-drawer button{display:inline-flex;align-items:center;justify-content:center;gap:6px;background:transparent;color:var(--ink);border:0;border-radius:5px;padding:8px}.category-drawer button:hover{background:var(--soft)}.category-description{font-size:13px;line-height:1.8;color:var(--muted);margin:16px 0 24px}.category-tools{display:flex;gap:12px;margin-bottom:16px}.category-tools label{display:flex;align-items:center;gap:8px;flex:1;border:1px solid var(--line);border-radius:6px;padding:0 10px}.category-tools input{width:100%;min-width:0;border:0;background:transparent;padding:10px 0;color:var(--ink)}.category-row{display:flex;align-items:center;border-bottom:1px solid var(--line);gap:4px}.category-drawer .category-choice{flex:1;justify-content:space-between;text-align:left;padding:14px 8px;width:100%}.category-choice[aria-pressed=true]{color:var(--green)}.category-choice small{display:block;font-size:11px;color:var(--muted);margin-top:4px}.category-drawer form{display:grid;gap:16px;margin-top:12px}.category-drawer form input{height:40px;padding:0 12px;border:1px solid var(--line);border-radius:6px;background:var(--bg);color:var(--ink)}.category-drawer footer{display:flex;justify-content:flex-end;gap:8px}.category-drawer footer button:last-child{background:var(--ink);color:var(--paper)}.category-drawer footer .danger{background:var(--error-ink,#b43e3e)}[role=alert]{font-size:13px;color:var(--error-ink,#b43e3e)}.muted{font-size:13px;color:var(--muted)}button:disabled{opacity:.5}
</style>

<style scoped>@media (prefers-reduced-motion:no-preference){.category-drawer[data-state=open]{animation:category-enter .18s ease-out}}@keyframes category-enter{from{transform:translateX(100%)}to{transform:translateX(0)}}</style>
