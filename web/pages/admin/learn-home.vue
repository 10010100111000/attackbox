<script setup lang="ts">
const confirmAction = useConfirm()
import { Plus, ArrowUp, ArrowDown, Trash2 } from 'lucide-vue-next'
import { parseLearnHome, type LearnHome } from '~/shared/learn-home'
definePageMeta({layout:'admin'})
useHead({title:'首页管理 — AttackBox'})
const {data,pending,error,refresh}=await useLazyFetch<{data:LearnHome}>('/api/admin/learn-home')
const draft=ref<LearnHome|null>(null), selected=ref(0), busy=ref(false), message=ref(''), failed=ref(false)
const clone=(v:LearnHome)=>JSON.parse(JSON.stringify(v)) as LearnHome
watch(data,v=>{if(v&&!busy.value) draft.value=clone(v.data)},{immediate:true})
const dirty=computed(()=>!!draft.value&&JSON.stringify(draft.value)!==JSON.stringify(data.value?.data))
const item=computed(()=>draft.value?.slides[selected.value])
const {data:modules}=useLazyFetch<{data:{title:string;slug:string}[]}>('/api/modules')
const {data:routes}=useLazyFetch<{data:{title:string;id:string}[]}>('/api/routes')
function add(){if(!draft.value||draft.value.slides.length>=10)return;draft.value.slides.push({id:crypto.randomUUID(),enabled:true,label:'模块推荐',title:'',description:'',cta:'查看模块',href:'/learn',command:'$ explore',caption:'AttackBox Learning'});selected.value=draft.value.slides.length-1}
function remove(){if(!draft.value)return;draft.value.slides.splice(selected.value,1);selected.value=Math.max(0,selected.value-1)}
function move(delta:number){const a=draft.value?.slides;if(!a)return;const n=selected.value+delta;if(n<0||n>=a.length)return;[a[selected.value],a[n]]=[a[n],a[selected.value]];selected.value=n}
function reset(){if(data.value)draft.value=clone(data.value.data);selected.value=0;message.value=''}
async function reload(){if(dirty.value&&!(await confirmAction({title:'重新载入首页配置？',description:'当前未保存的修改将被替换。',confirmLabel:'重新载入',cancelLabel:'继续编辑'})))return;await refresh();if(!error.value)reset()}
async function save(){if(!draft.value||busy.value)return;busy.value=true;message.value='';try{const body=parseLearnHome(draft.value);const result=await $fetch<{data:LearnHome}>('/api/admin/learn-home',{method:'PUT',body,retry:false});data.value=result;draft.value=clone(result.data);failed.value=false;message.value='首页已更新'}catch(e:any){failed.value=true;message.value=e?.data?.message||e.message||'保存失败，修改已保留'}finally{busy.value=false}}
onBeforeRouteLeave(()=>!dirty.value||confirmAction({title:'放弃首页修改？',description:'当前未保存的首页配置将丢失。',confirmLabel:'放弃修改',cancelLabel:'继续编辑'}))
function beforeUnload(e:BeforeUnloadEvent){if(dirty.value){e.preventDefault();e.returnValue=''}}
onMounted(()=>window.addEventListener('beforeunload',beforeUnload));onUnmounted(()=>window.removeEventListener('beforeunload',beforeUnload))
</script>
<template>
 <div class="home-manager">
  <header><div><h1>首页管理</h1><p>管理学习首页的推荐内容，保存后直接生效。</p></div><NuxtLink to="/learn" target="_blank" rel="noopener">查看学习首页</NuxtLink></header>
  <UiBusyIndicator v-if="pending&&!draft" label="首页设置加载"/><p v-else-if="error" role="alert">读取失败。<button @click="reload">重试</button></p>
  <form v-else-if="draft" @submit.prevent="save">
   <fieldset :disabled="busy">
    <section class="settings"><label><input v-model="draft.enabled" type="checkbox">显示推荐区</label><label><input v-model="draft.autoplay" type="checkbox">自动轮播</label><label>间隔 <input v-model.number="draft.intervalSeconds" type="number" min="5" max="30" required> 秒</label></section>
    <div class="workspace">
     <aside><div class="list-heading"><h2>推荐内容 <small>{{ draft.slides.length }}/10</small></h2><button type="button" :disabled="draft.slides.length>=10" aria-label="添加推荐" @click="add"><Plus :size="18"/></button></div><button v-for="(s,i) in draft.slides" :key="s.id" type="button" class="slide-row" :class="{selected:selected===i}" @click="selected=i"><span>{{ String(i+1).padStart(2,'0') }}</span><strong>{{ s.title||'未命名推荐' }}</strong><small>{{ s.enabled?'显示':'隐藏' }}</small></button><p v-if="!draft.slides.length">暂无推荐，点击添加开始。</p></aside>
     <section v-if="item" class="editor"><div class="list-heading"><h2>编辑推荐</h2><div class="tools"><button type="button" aria-label="上移" :disabled="selected===0" @click="move(-1)"><ArrowUp :size="17"/></button><button type="button" aria-label="下移" :disabled="selected===draft.slides.length-1" @click="move(1)"><ArrowDown :size="17"/></button><button type="button" aria-label="删除推荐" @click="remove"><Trash2 :size="17"/></button></div></div>
      <div class="fields"><label class="wide">标题 *<input v-model="item.title" maxlength="100" required></label><label>标签<input v-model="item.label" maxlength="30"></label><label class="toggle"><input v-model="item.enabled" type="checkbox">在首页显示</label><label class="wide">说明<textarea v-model="item.description" maxlength="300" rows="3"/></label><label>按钮文字 *<input v-model="item.cta" maxlength="30" required></label><label>跳转路径 *<input v-model="item.href" list="home-destinations" maxlength="300" required><small>选择已发布模块或路线，也可填写本站路径。</small></label><label>终端装饰文字<input v-model="item.command" maxlength="100"></label><label>终端说明<input v-model="item.caption" maxlength="100"></label></div>
      <datalist id="home-destinations"><option v-for="m in modules?.data" :key="m.slug" :value="`/learn/modules/${m.slug}`">{{ m.title }}</option><option v-for="r in routes?.data" :key="r.id" :value="`/learn/routes/${r.id}`">{{ r.title }}</option></datalist>
     </section>
    </div>
    <section v-if="item" class="preview"><h2>当前项预览 <small>未保存的修改仅在此处展示</small></h2><LearningHomeCarousel :slides="[item]" :autoplay="false" :interval-seconds="draft.intervalSeconds"/></section>
   </fieldset>
   <footer><p :role="failed?'alert':'status'">{{ message|| (dirty?'有未保存的修改':'') }}</p><button type="button" :disabled="busy||!dirty" @click="reset">取消修改</button><button type="button" :disabled="busy" @click="reload">重新载入</button><button class="primary" :disabled="busy||!dirty"><UiBusyLabel :busy="busy">保存首页</UiBusyLabel></button></footer>
  </form>
 </div>
</template>
<style scoped>
.home-manager{color:var(--ink);padding:32px;max-width:1400px;margin:auto}header,.list-heading,footer,.tools,.settings{display:flex;align-items:center;gap:16px}header{justify-content:space-between;margin-bottom:28px}h1{font-size:26px;margin:0 0 8px}h2{font-size:15px;margin:0}p,small{color:var(--muted)}small{font-size:12px;font-weight:400}header p{margin:0}fieldset{border:0;padding:0;margin:0;min-width:0}button,input,textarea{font:inherit}button{cursor:pointer;border:1px solid var(--line);background:var(--paper);color:var(--ink);border-radius:7px;padding:8px 12px}button:disabled{opacity:.45;cursor:default}input:not([type=checkbox]),textarea{border:1px solid var(--line);border-radius:6px;background:var(--paper);color:var(--ink);padding:9px 11px;min-width:0;width:100%;box-sizing:border-box}.settings{padding:18px 22px;background:var(--paper);border:1px solid var(--line);border-radius:10px;margin-bottom:20px}.settings label{display:flex;align-items:center;gap:8px}.settings input[type=number]{width:72px}.workspace{display:grid;grid-template-columns:300px minmax(0,1fr);border:1px solid var(--line);background:var(--paper);border-radius:10px;overflow:hidden;min-height:350px}aside{border-right:1px solid var(--line);padding:20px}.list-heading{justify-content:space-between;margin-bottom:20px}.slide-row{display:flex;width:100%;gap:10px;text-align:left;margin-bottom:8px;align-items:center;border-color:transparent}.slide-row strong{flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.slide-row span{font-size:12px;color:var(--muted)}.slide-row.selected{background:var(--soft);border-color:var(--line)}.editor{padding:22px}.tools{gap:6px}.fields{display:grid;grid-template-columns:1fr 1fr;gap:18px}.fields label{display:flex;flex-direction:column;gap:7px;font-size:13px}.fields .toggle{flex-direction:row;align-items:center}.wide{grid-column:1/-1}.preview{margin-top:26px}.preview h2{margin-bottom:14px}footer{margin-top:22px}footer p{flex:1}.primary{background:var(--ink);color:var(--paper)}a{color:var(--muted)}
</style>
