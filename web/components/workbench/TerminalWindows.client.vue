<script setup lang="ts">
const confirmAction = useConfirm()
import {ref,onMounted,onBeforeUnmount} from 'vue'
import {Plus,LayoutGrid,Layers,Minus,Maximize2,X} from 'lucide-vue-next'
import {LearningSandboxTerminal as SandboxTerminal} from '#components'
defineProps<{sandboxId:string}>()
type WindowState={id:string;name:string;x:number;y:number;width:number;height:number;z:number;minimized:boolean;maximized:boolean}
const windows=ref<WindowState[]>([]),area=ref<HTMLElement|null>(null),focused=ref('')
let sequence=0,z=0,drag:{window:WindowState;dx:number;dy:number;target:HTMLElement;pointer:number}|null=null
function focus(w:WindowState){w.z=++z;focused.value=w.id}
function add(){
 const width=area.value?.clientWidth||1000,height=area.value?.clientHeight||700
 const offset=(sequence%6)*26
 const w:WindowState={id:globalThis.crypto?.randomUUID?.()||`terminal-${Date.now()}-${++z}`,name:`终端 ${++sequence}`,x:Math.min(24+offset,Math.max(0,width-360)),y:Math.min(20+offset,Math.max(0,height-240)),width:Math.min(850,width),height:Math.min(500,height),z:++z,minimized:false,maximized:false}
 windows.value.push(w);focused.value=w.id
}
async function closeAll(){if(await confirmAction({title:'关闭全部终端？',description:'仅关闭终端连接，沙箱环境仍会保留。',confirmLabel:'关闭终端',tone:'neutral'})){windows.value=[];focused.value=''}}
function close(w:WindowState){windows.value=windows.value.filter(item=>item.id!==w.id);if(focused.value===w.id)focused.value=windows.value.at(-1)?.id||''}
function arrange(tile:boolean){
 const width=area.value?.clientWidth||1000,height=area.value?.clientHeight||700
 const cols=Math.ceil(Math.sqrt(windows.value.length||1)),rows=Math.ceil(windows.value.length/cols)
 windows.value.forEach((w,i)=>{w.minimized=false;w.maximized=false;w.x=tile?(i%cols)*width/cols:20+(i%8)*28;w.y=tile?Math.floor(i/cols)*height/rows:16+(i%8)*28;w.width=tile?width/cols:Math.min(850,width-w.x);w.height=tile?height/rows:Math.min(500,height-w.y)})
}
function begin(e:PointerEvent,w:WindowState){if(e.button!==0||w.maximized)return;focus(w);const target=e.currentTarget as HTMLElement;target.setPointerCapture(e.pointerId);drag={window:w,dx:e.clientX-w.x,dy:e.clientY-w.y,target,pointer:e.pointerId};e.preventDefault()}
function move(e:PointerEvent){if(!drag)return;drag.window.x=Math.max(0,Math.min(e.clientX-drag.dx,(area.value?.clientWidth||1000)-120));drag.window.y=Math.max(0,Math.min(e.clientY-drag.dy,(area.value?.clientHeight||700)-38))}
function end(){if(drag?.target.hasPointerCapture(drag.pointer))drag.target.releasePointerCapture(drag.pointer);drag=null}
function key(e:KeyboardEvent){if(e.ctrlKey&&e.altKey&&e.code==='KeyT'){e.preventDefault();add()}}
function reposition(){for(const w of windows.value){w.x=Math.min(w.x,Math.max(0,(area.value?.clientWidth||1000)-120));w.y=Math.min(w.y,Math.max(0,(area.value?.clientHeight||700)-38))}}
onMounted(()=>{add();window.addEventListener('keydown',key);window.addEventListener('resize',reposition)})
onBeforeUnmount(()=>{end();window.removeEventListener('keydown',key);window.removeEventListener('resize',reposition)})
</script>
<template><div class="terminal-desktop"><nav aria-label="终端窗口管理"><button title="新建终端（Ctrl+Alt+T）" @click="add"><Plus :size="15"/>新建终端</button><button :disabled="!windows.length" @click="arrange(true)"><LayoutGrid :size="15"/>平铺</button><button :disabled="!windows.length" @click="arrange(false)"><Layers :size="15"/>层叠</button><button v-if="windows.length" @click="closeAll">全部关闭</button><button v-for="w in windows.filter(w=>w.minimized)" :key="w.id" @click="w.minimized=false;focus(w)">{{w.name}}</button></nav><div ref="area" class="window-area"><p v-if="!windows.length" class="window-empty">关闭窗口不会释放环境。<button @click="add">新建终端</button></p><section v-for="w in windows" v-show="!w.minimized" :key="w.id" class="terminal-window" :class="{maximized:w.maximized,focused:focused===w.id}" :style="w.maximized?{zIndex:w.z}:{left:w.x+'px',top:w.y+'px',width:w.width+'px',height:w.height+'px',zIndex:w.z}" :aria-label="w.name" @pointerdown="focus(w)"><header @pointerdown="begin($event,w)" @pointermove="move" @pointerup="end" @pointercancel="end" @lostpointercapture="end" @dblclick="w.maximized=!w.maximized"><span>{{w.name}}</span><div @pointerdown.stop @dblclick.stop><button :aria-label="`最小化 ${w.name}`" @click="w.minimized=true"><Minus :size="14"/></button><button :aria-label="`展开或还原 ${w.name}`" @click="focus(w);w.maximized=!w.maximized"><Maximize2 :size="13"/></button><button :aria-label="`关闭 ${w.name}`" @click="close(w)"><X :size="14"/></button></div></header><div class="window-body"><SandboxTerminal :sandbox-id="sandboxId" :window-id="w.id" :active="focused===w.id && !w.minimized"/></div></section></div></div></template>
<style scoped>.terminal-desktop{height:100%;display:flex;flex-direction:column;min-height:0;background:var(--bg)}nav{display:flex;gap:8px;align-items:center;padding:8px 16px;min-height:44px;border-bottom:1px solid var(--line);flex-wrap:wrap}button{display:inline-flex;gap:6px;align-items:center;justify-content:center;padding:6px 10px;border:0;border-radius:5px;background:transparent;color:var(--muted);cursor:pointer;font-size:12px}button:hover{background:var(--soft);color:var(--ink)}button:disabled{opacity:.4}.window-area{position:relative;flex:1;min-height:0;overflow:hidden}.terminal-window{position:absolute;display:flex;flex-direction:column;border:1px solid var(--line);border-radius:8px;box-shadow:0 12px 32px #0003;overflow:hidden;resize:both;min-width:260px;min-height:180px;background:#101419}.terminal-window.focused{border-color:var(--green)}.terminal-window.maximized{inset:0;border-radius:0;resize:none;width:100%;height:100%}.terminal-window header{display:flex;align-items:center;justify-content:space-between;flex-shrink:0;height:34px;padding:0 6px 0 12px;background:var(--paper);color:var(--ink);font-size:12px;cursor:move;touch-action:none;user-select:none}.terminal-window header div{display:flex}.window-body{flex:1;min-height:0;min-width:0}.window-empty{padding:48px;text-align:center;color:var(--muted);font-size:13px}</style>
