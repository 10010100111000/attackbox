<script setup lang="ts">
import {ref,onMounted,onBeforeUnmount} from 'vue'
const props=defineProps<{context:Record<string,string>}>()
const current=ref<any>(null),busy=ref(false),message=ref(''),mounted=ref(false)
let timer:ReturnType<typeof setInterval>|undefined
async function run(action:'inspect'|'start'|'stop'){
 if(busy.value)return
 busy.value=true;message.value=''
 try{
  const result=action==='inspect'?await $fetch('/api/targets',{query:props.context}):await $fetch('/api/targets',{method:'POST',body:{...props.context,action,id:current.value?.id}})
  current.value=result;message.value=(result as any)?.error||''
 }catch(e:any){message.value=e?.data?.message||'环境操作失败，请稍后重试'}finally{busy.value=false}
}
onMounted(()=>{mounted.value=true;run('inspect');timer=setInterval(()=>{if(current.value && current.value.state!=='stopped')run('inspect')},8000)})
onBeforeUnmount(()=>clearInterval(timer))
</script>
<template><section class="target-runtime" aria-label="靶机运行状态"><div><strong>实验靶机</strong><span v-if="current">{{current.instance?.ready?'服务已就绪':current.state}}</span></div><p v-if="current?.instance?.ip"><code>{{current.instance.ip}}</code><small>使用同节点 CLI / GUI 访问</small></p><p v-if="message" role="status">{{message}}</p><footer><button :disabled="!mounted||busy||current?.state==='stopping'" @click="run('start')">{{busy?'处理中…':current && current.state!=='stopped'?'继续启动／重试':'启动靶机'}}</button><button v-if="current && current.state!=='stopped'" :disabled="busy" @click="run('stop')">停止靶机</button><button :disabled="!mounted||busy" @click="run('inspect')">刷新状态</button></footer></section></template>
<style scoped>.target-runtime{margin:18px 0;padding:18px;border:1px solid var(--line);border-radius:8px;background:var(--paper);font-size:13px}.target-runtime>div{display:flex;justify-content:space-between;gap:20px}.target-runtime p{line-height:1.8}.target-runtime small{margin-left:16px;color:var(--muted)}footer{display:flex;gap:10px;margin-top:16px}button{border:1px solid var(--line);border-radius:6px;padding:9px 14px;background:var(--paper);color:var(--ink);cursor:pointer}button:first-child{background:var(--ink);color:var(--paper)}button:disabled{opacity:.5;cursor:default}</style>
