import {computed,onMounted,onBeforeUnmount,watch} from 'vue'
import {useNuxtApp,useState} from '#imports'
import {useAuth} from './useAuth'
import type {SandboxStatus,SandboxInstance} from '~/shared/sandbox'
const runtimes=new WeakMap<object,{users:number;generation:number;fetching:boolean;timer?:ReturnType<typeof setInterval>;clock?:ReturnType<typeof setInterval>;channel?:BroadcastChannel;refresh:()=>Promise<void>;client:string;focus?:()=>void}>()
export function useSandboxSession(){
 const app=useNuxtApp(),{currentUser}=useAuth()
 const sandbox=useState<SandboxStatus>('sandbox:session',()=>({available:false,instance:null}))
 const loading=useState('sandbox:loading',()=>true),busy=useState('sandbox:busy',()=>false),notice=useState('sandbox:notice',()=>''),now=useState('sandbox:clock',Date.now),guiElsewhere=useState('sandbox:gui-elsewhere',()=>false),owner=useState('sandbox:owner',()=>currentUser.value?.id||'')
 let rt=runtimes.get(app)
 if(!rt){rt={users:0,generation:0,fetching:false,refresh:async()=>{},client:''};runtimes.set(app,rt)}
 const runtime=rt
 function identity(){const id=currentUser.value?.id||'';if(owner.value!==id){owner.value=id;runtime.generation++;sandbox.value={available:false,instance:null};guiElsewhere.value=false;notice.value='';loading.value=!!id}}
 async function refresh(){
  identity()
  if(!currentUser.value){loading.value=false;return}
  if(busy.value||runtime.fetching)return
  runtime.fetching=true;const generation=runtime.generation
  try{const r=await $fetch<{data:SandboxStatus}>('/api/sandbox/status');if(generation===runtime.generation){sandbox.value=r.data;notice.value=''}}
  catch(e:any){if(generation===runtime.generation)notice.value=e?.data?.message||'无法获取环境状态'}
  finally{runtime.fetching=false;if(generation===runtime.generation)loading.value=false;else if(currentUser.value)void runtime.refresh()}
 }
 runtime.refresh=refresh
 function notify(){runtime.channel?.postMessage({type:'status',owner:owner.value})}
 async function start(){
  if(busy.value||!currentUser.value)return
  busy.value=true;runtime.generation++;notice.value='';const generation=runtime.generation
  try{const r=await $fetch<{data:SandboxInstance}>('/api/sandbox/start',{method:'POST'});if(generation===runtime.generation)sandbox.value={available:true,instance:r.data};notify()}
  catch(e:any){if(generation===runtime.generation)notice.value=e?.data?.message||'启动失败，请刷新状态重试'}finally{busy.value=false;await refresh()}
 }
 async function release(){
  const id=sandbox.value.instance?.id
  if(!id||busy.value||!confirm('释放环境会关闭所有终端和桌面，并清理沙箱文件与进程。确认释放？'))return
  busy.value=true;runtime.generation++;notice.value='';const generation=runtime.generation
  try{const r=await $fetch<{data:SandboxStatus}>('/api/sandbox/stop',{method:'POST',body:{sandboxId:id}});if(generation===runtime.generation)sandbox.value=r.data;notify()}
  catch(e:any){if(generation===runtime.generation)notice.value=e?.data?.message||'释放尚未确认，请刷新状态'}finally{busy.value=false;await refresh()}
 }
 function announceGUI(){guiElsewhere.value=false;runtime.channel?.postMessage({type:'gui-active',id:sandbox.value.instance?.id,owner:owner.value,client:runtime.client})}
 function resumeGUI(){guiElsewhere.value=false}
 function openViewer(mode:'cli'|'gui'){
  const path=mode==='cli'?'/term':'/gui'
  const viewer=window.open('',mode==='cli'?'attackbox_cli':'attackbox_gui')
  if(!viewer){notice.value='浏览器阻止了新窗口，请允许本站弹出窗口';return}
  try{if(viewer.location.origin!==window.location.origin||viewer.location.pathname!==path)viewer.location.replace(path)}catch{viewer.location=path}
  viewer.focus()
 }
  onMounted(()=>{
  identity();runtime.users++
  if(runtime.users===1){
   runtime.client=globalThis.crypto?.randomUUID?.()||`${Date.now()}-${Math.random()}`
   if(typeof BroadcastChannel!=='undefined'){
    runtime.channel=new BroadcastChannel('attackbox:environment')
    runtime.channel.onmessage=e=>{const m=e.data;if(m?.owner!==owner.value)return;if(m.type==='status')void runtime.refresh();if(m.type==='gui-active'&&m.id===sandbox.value.instance?.id&&m.client!==runtime.client)guiElsewhere.value=true}
   }
   runtime.focus=()=>{if(!document.hidden)void runtime.refresh()}
   runtime.timer=setInterval(runtime.focus,10000);runtime.clock=setInterval(()=>{now.value=Date.now()},1000);window.addEventListener('focus',runtime.focus)
  }
  void refresh()
 })
 onBeforeUnmount(()=>{runtime.users--;if(runtime.users===0){clearInterval(runtime.timer);clearInterval(runtime.clock);runtime.channel?.close();runtime.channel=undefined;if(runtime.focus)window.removeEventListener('focus',runtime.focus)}})
 watch(()=>sandbox.value.instance?.id,()=>{guiElsewhere.value=false})
 watch(()=>currentUser.value?.id,()=>{identity();if(import.meta.client)void refresh()})
 const seconds=computed(()=>sandbox.value.instance?.expiresAt?Math.max(0,Math.ceil((Date.parse(sandbox.value.instance.expiresAt)-now.value)/1000)):null)
 const countdown=computed(()=>{const n=seconds.value;if(n===null)return '';return `${Math.floor(n/60).toString().padStart(2,'0')}:${(n%60).toString().padStart(2,'0')}`})
 return {sandbox,loading,busy,notice,countdown,seconds,guiElsewhere,refresh,start,release,openViewer,announceGUI,resumeGUI,currentUser}
}
