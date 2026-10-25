<script setup lang="ts">
import {computed} from 'vue'
import {Power, Square, LoaderCircle} from 'lucide-vue-next'
import {useSandboxSession} from '~/composables/useSandboxSession'
const {sandbox,loading,busy,notice,countdown,start,release,currentUser}=useSandboxSession()
const instance=computed(()=>sandbox.value.instance)
const pending=computed(()=>busy.value || loading.value || instance.value?.state==='creating' || instance.value?.state==='stopping')
const label=computed(()=>{
 if(loading.value)return '读取环境…'
 if(instance.value?.state==='stopping')return '正在释放…'
 if(instance.value?.state==='creating')return '正在启动…'
 if(busy.value)return instance.value?'正在释放…':'正在启动…'
 if(instance.value?.state==='cleanup_failed')return '重试释放'
 if(instance.value)return instance.value.connected?'停止环境':'节点断开 · 释放环境'
 return sandbox.value.available?'启动环境':'环境暂不可用'
})
function toggle(){if(instance.value)void release();else void start()}
</script>
<template>
 <div class="environment-status">
  <button v-if="currentUser" type="button" class="environment-toggle" :class="{running:instance?.state==='running',cleanup:instance?.state==='cleanup_failed'}" :disabled="pending || (!instance && !sandbox.available)" :aria-label="instance?.state==='running'?'停止环境':label" :aria-busy="pending" @click="toggle">
   <LoaderCircle v-if="pending" :size="14" class="spinning" aria-hidden="true"/>
   <Square v-else-if="instance" :size="12" aria-hidden="true"/>
   <Power v-else :size="14" aria-hidden="true"/>
   <span>{{label}}</span>
   <time v-if="instance?.state==='running' && countdown" :datetime="instance.expiresAt||undefined">{{countdown}}</time>
  </button>
  <NuxtLink v-else to="/auth" class="environment-toggle">登录后启动环境</NuxtLink>
  <span v-if="notice || instance?.error" class="environment-error" role="alert">{{notice || instance?.error}}</span>
 </div>
</template>
<style scoped>
.environment-status{display:flex;align-items:center;gap:10px;min-width:0}
.environment-toggle{display:inline-flex;align-items:center;justify-content:center;gap:7px;min-height:32px;padding:7px 12px;border:1px solid var(--green);border-radius:6px;background:var(--green);color:var(--on-accent);font-size:12px;font-weight:600;white-space:nowrap;cursor:pointer;text-decoration:none;transition:background .15s,border-color .15s}
.environment-toggle.running,.environment-toggle.cleanup{color:var(--environment-danger,#b42336);border-color:currentColor;background:var(--environment-danger-bg,#fff0f1)}
:global([data-theme="midnight"]) .environment-toggle.running,:global([data-theme="midnight"]) .environment-toggle.cleanup{--environment-danger:#ff9da8;--environment-danger-bg:#39252c}
.environment-toggle:hover:not(:disabled){filter:brightness(.94)}
.environment-toggle:focus-visible{outline:2px solid var(--green);outline-offset:3px}
.environment-toggle:disabled{cursor:default;opacity:.6}
time{font-variant-numeric:tabular-nums;border-left:1px solid currentColor;padding-left:8px;font-weight:650}
.environment-error{max-width:300px;color:var(--error-ink,#b42336);font-size:11px;line-height:1.5}
.spinning{animation:environment-spin 1s linear infinite}@keyframes environment-spin{to{transform:rotate(360deg)}}
@media(prefers-reduced-motion:reduce){.spinning{animation:none}}
</style>
