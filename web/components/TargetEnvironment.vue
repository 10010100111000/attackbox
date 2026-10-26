<script setup lang="ts">
import {ref,computed,onMounted,onBeforeUnmount,watch} from 'vue'
import {Box,Copy,Check,Power,Square,LoaderCircle,ChevronDown} from 'lucide-vue-next'
import type {LearningTarget} from '~/shared/learning-target'
const props=defineProps<{context:Record<string,string>;target?:LearningTarget}>()
const {currentUser}=useAuth()
const sandboxSession=useState<import('~/shared/sandbox').SandboxStatus>('sandbox:session',()=>({available:false,instance:null}))
interface Instance {expiresAt:string;id:string;state:string;instance?:{ip?:string;entry_address?:string;attacker_address?:string;interface?:string;attachment?:string;services?:import('~/shared/compose-labs').ComposeService[];ready:boolean;readiness:string};error?:string}
const requestPending=ref(false)
const current=ref<Instance|null>(null),initializing=ref(true),action=ref(''),message=ref(''),copied=ref('')
let alive=true,copyTimer:ReturnType<typeof setTimeout>|undefined
let identityGeneration=0
const clock=ref(Date.now())
let clockTimer:ReturnType<typeof setInterval>|undefined
const remaining=computed(()=>current.value?.expiresAt?Math.max(0,Math.ceil((Date.parse(current.value.expiresAt)-clock.value)/1000)):null)
const countdown=computed(()=>remaining.value===null?'':`${Math.floor(remaining.value/60).toString().padStart(2,'0')}:${(remaining.value%60).toString().padStart(2,'0')}`)
const config=computed(()=>props.target?.status==='configured'?props.target:null)
const unavailable=computed(()=>props.target?.status==='unavailable')
const state=computed(()=>current.value?.state||'stopped')
const address=computed(()=>current.value?.instance?.entry_address||current.value?.instance?.ip||'')
const attackerAddress=computed(()=>current.value?.instance?.attachment==='connected'?current.value.instance.attacker_address||'':'')
const active=computed(()=>!!current.value && state.value!=='stopped')
const canRetry=computed(()=>['missing','created'].includes(state.value))
const status=computed(()=>{
 if(initializing.value)return '正在读取状态'
 if(active.value && remaining.value===0)return '租期已到期'
 if(action.value==='start')return '正在启动'
 if(action.value==='stop')return '正在停止'
 if(state.value==='running'&&current.value?.instance?.attachment==='not_connected')return '服务运行中 · 尚未接入攻击环境'
 if(current.value?.instance?.ready && state.value==='running')return '已就绪'
 if(state.value==='running')return '运行中'
 return ({starting:'正在启动',incomplete:'启动未完成，请释放后重试',degraded:'部分服务未就绪',failed:'服务异常，请释放后重试',created:'尚未启动',stopping:'正在停止',cleanup_pending:'清理待确认',missing:'实例缺失',orphaned:'状态异常',unconfirmed:'状态待确认',stopped:'未启动'} as Record<string,string>)[state.value]||'状态待确认'
})
async function run(operation:'inspect'|'start'|'stop'){
 if(requestPending.value || action.value || !currentUser.value || (unavailable.value&&operation==='start'))return
 if(operation==='start' && active.value && remaining.value===0)return
 if(operation==='stop'&&!confirm('停止靶机会清除其中的修改，确认停止？'))return
 requestPending.value=true
 const generation=identityGeneration
 if(operation!=='inspect'){action.value=operation;message.value=''}
 try{
  const result=operation==='inspect'?await $fetch<Instance|null>('/api/targets',{query:props.context,retry:false}):await $fetch<Instance>('/api/targets',{method:'POST',body:{...props.context,action:operation,id:current.value?.id}})
  if(alive&&generation===identityGeneration){current.value=result;message.value=result?.error||''}
 }catch(e:any){if(alive&&generation===identityGeneration)message.value=e?.data?.message||'操作结果尚未确认'}
 finally{if(alive&&generation===identityGeneration){requestPending.value=false;action.value='';initializing.value=false}}
}
async function copyText(value:string){
 try{await navigator.clipboard.writeText(value);copied.value=value;clearTimeout(copyTimer);copyTimer=setTimeout(()=>copied.value='',2000)}catch{message.value='复制失败，请手动复制地址'}
}
onMounted(()=>{clockTimer=setInterval(()=>clock.value=Date.now(),1000);initializing.value=!!currentUser.value && !unavailable.value;void run('inspect')})
// A sandbox identity transition is an action/lifecycle event, not a polling loop.
watch(()=>sandboxSession.value.instance?.id,()=>{if(active.value&&current.value?.instance?.attachment)void run('inspect')})
watch(()=>currentUser.value?.id,()=>{identityGeneration++;current.value=null;message.value='';action.value='';requestPending.value=false;initializing.value=!!currentUser.value;if(currentUser.value)void run('inspect')})
onBeforeUnmount(()=>{alive=false;clearInterval(clockTimer);clearTimeout(copyTimer)})
</script>
<template>
 <section class="target-runtime" aria-label="实验靶机">
  <div class="target-row">
   <div class="target-heading"><Box :size="17" aria-hidden="true"/><div><span class="target-label">实验靶机</span><h2>{{config?.name || '实验环境'}}</h2></div></div>
   <span v-if="!unavailable && currentUser" class="target-state" :class="{ready:current?.instance?.ready && state==='running' && remaining!==0}" role="status"><i/>{{status}}</span>
   <time v-if="active && countdown" class="target-countdown" :datetime="current?.expiresAt" title="到期自动释放">{{countdown}}</time>
   <div class="target-action">
    <span v-if="unavailable&&!active" class="target-state">暂不可用</span>
    <NuxtLink v-else-if="!currentUser" to="/auth">登录后启动</NuxtLink>
    <template v-else>
     <button v-if="(!active || canRetry)&&!unavailable" type="button" :disabled="initializing || !!action || requestPending || (active && remaining===0)" class="start" @click="run('start')"><LoaderCircle v-if="action==='start'" :size="14" aria-hidden="true"/><Power v-else :size="14" aria-hidden="true"/>{{canRetry?'重试启动':'启动靶机'}}</button>
     <button v-if="active" type="button" :disabled="!!action || requestPending" class="stop" @click="run('stop')"><Square :size="12" aria-hidden="true"/>{{state==='cleanup_pending'?'重试停止':'停止靶机'}}</button>
    </template>
   </div>
  </div>
  <div v-if="address && state==='running'" class="target-address"><span>目标地址</span><code>{{address}}</code><button type="button" :aria-label="copied===address?'已复制目标地址':'复制目标地址'" @click="copyText(address)"><Check v-if="copied===address" :size="14"/><Copy v-else :size="14"/></button></div>
  <div v-if="attackerAddress && state==='running'" class="target-address"><span title="反向连接时使用此地址">回连地址</span><code>{{attackerAddress}}</code><button type="button" :aria-label="copied===attackerAddress?'已复制回连地址':'复制回连地址'" @click="copyText(attackerAddress)"><Check v-if="copied===attackerAddress" :size="14"/><Copy v-else :size="14"/></button><span>{{current?.instance?.interface}}</span></div>
  <details v-if="current?.instance?.services?.length" class="target-details"><summary>服务状态<ChevronDown :size="13" aria-hidden="true"/></summary><div><p v-for="service in current.instance.services" :key="service.name"><strong>{{service.name}}</strong> · {{({running:'运行中',exited:'已退出',failed:'异常',created:'等待启动'} as Record<string,string>)[service.state]||service.state}}<span v-if="service.health==='unhealthy'"> · 健康检查未通过</span></p></div></details>
  <p v-if="message" class="target-message" role="alert">{{message}}</p>
  <details v-if="config && (config.description || config.accessInstructions || config.ports.length)" class="target-details">
   <summary>环境说明<ChevronDown :size="13" aria-hidden="true"/></summary>
   <div><p v-if="config.description">{{config.description}}</p><p v-if="config.ports.length" class="ports">服务端口 · {{config.ports.map(p=>`${p.port}/${p.protocol.toUpperCase()}`).join('、')}}</p><p v-if="config.accessInstructions" class="instructions">{{config.accessInstructions}}</p><p class="cleanup-note">独享靶机，停止后清除其中的修改。</p></div>
  </details>
 </section>
</template>
<style scoped>
.target-runtime{margin:32px 0;padding:18px 20px;border:1px solid var(--line);border-radius:10px;background:var(--paper);color:var(--ink);font-size:13px}
.target-row{display:flex;align-items:center;gap:18px;flex-wrap:wrap}.target-heading{display:flex;align-items:center;gap:10px;min-width:0}.target-heading>svg{color:var(--muted);flex-shrink:0}.target-label{font-size:10px;color:var(--muted)}h2{margin:3px 0 0;font-size:14px;font-weight:600;overflow-wrap:anywhere}.target-state{display:inline-flex;gap:6px;align-items:center;font-size:11px;color:var(--muted)}.target-state i{height:5px;width:5px;border-radius:50%;background:currentColor}.target-state.ready{color:var(--green)}.target-countdown{color:var(--error-ink,#b44949);font-size:12px;font-variant-numeric:tabular-nums}.target-action{margin-left:auto;display:flex;align-items:center;gap:8px}
button,.target-action a{display:inline-flex;align-items:center;gap:6px;border:1px solid var(--line);background:transparent;color:var(--ink);border-radius:6px;padding:7px 10px;cursor:pointer;font-size:12px;text-decoration:none}.start{background:var(--green);border-color:var(--green);color:var(--on-accent)}.stop{color:var(--error-ink,#b44949)}button:disabled{opacity:.5;cursor:default}button:focus-visible,summary:focus-visible{outline:2px solid var(--green);outline-offset:3px}.target-address{display:flex;align-items:center;gap:10px;margin-top:15px}.target-address>span{font-size:11px;color:var(--muted)}code{font-family:ui-monospace,monospace;font-size:13px;user-select:all}.target-address button{padding:5px;border:0}.target-message{color:var(--error-ink,#b44949);font-size:12px;line-height:1.6;margin:12px 0 0;overflow-wrap:anywhere}.target-details{margin-top:14px}.target-details summary{display:inline-flex;gap:6px;align-items:center;list-style:none;cursor:pointer;color:var(--muted);font-size:11px}.target-details summary::-webkit-details-marker{display:none}.target-details[open] summary svg{transform:rotate(180deg)}.target-details>div{border-top:1px solid var(--line);margin-top:12px;padding-top:4px}.target-details p{font-size:12px;line-height:1.8;margin:10px 0 0;overflow-wrap:anywhere}.instructions{white-space:pre-wrap}.ports,.cleanup-note{color:var(--muted)}
</style>
