<script setup lang="ts">
import {Trophy,X} from 'lucide-vue-next'
import {DialogRoot,DialogPortal,DialogOverlay,DialogContent,DialogTitle,DialogDescription,DialogClose} from 'reka-ui'
defineProps<{open:boolean;moduleTitle:string;moduleSlug:string;chapterCount:number}>()
const emit=defineEmits<{'update:open':[value:boolean]}>()
</script>
<template>
 <DialogRoot :open="open" @update:open="emit('update:open',$event)">
  <DialogPortal>
   <DialogOverlay class="completion-overlay"/>
   <DialogContent class="completion-dialog">
    <DialogClose class="dialog-close" aria-label="关闭完成提示"><X :size="18"/></DialogClose>
    <div class="trophy"><Trophy :size="32" :stroke-width="1.5" aria-hidden="true"/></div>
    <span class="completion-eyebrow">MODULE COMPLETE</span>
    <DialogTitle class="completion-title">完成了，做得好。</DialogTitle>
    <DialogDescription class="completion-description">你已完成「{{moduleTitle}}」的全部 {{chapterCount}} 个章节。</DialogDescription>
    <NuxtLink :to="`/learn/modules/${moduleSlug}`" class="completion-link" @click="emit('update:open',false)">返回模块</NuxtLink>
    <DialogClose class="stay-button">留在本章</DialogClose>
   </DialogContent>
  </DialogPortal>
 </DialogRoot>
</template>
<style scoped>
.completion-overlay{position:fixed;inset:0;background:#0007;z-index:140;backdrop-filter:blur(4px)}
.completion-dialog{position:fixed;left:50%;top:50%;transform:translate(-50%,-50%);z-index:141;width:min(420px,calc(100vw - 40px));padding:40px 32px 26px;border:1px solid var(--line);border-radius:16px;background:var(--paper);color:var(--ink);text-align:center;box-shadow:0 24px 80px #0003}
.dialog-close{position:absolute;right:14px;top:14px;padding:5px;border:0;background:transparent;color:var(--muted);cursor:pointer}.trophy{display:grid;place-items:center;margin:0 auto 24px;width:70px;height:70px;border:1px solid var(--line);border-radius:50%;background:var(--mint);color:var(--green)}.completion-eyebrow{font:10px ui-monospace,monospace;letter-spacing:2px;color:var(--muted)}.completion-title{font-size:26px;letter-spacing:-.6px;margin:12px 0}.completion-description{font-size:13px;line-height:1.9;color:var(--muted);margin:0 0 28px}.completion-link{display:block;border-radius:7px;background:var(--green);color:var(--on-accent);text-decoration:none;padding:12px;font-size:13px;font-weight:550}.stay-button{margin-top:14px;padding:6px 12px;background:transparent;border:0;color:var(--muted);font-size:12px;cursor:pointer}.completion-dialog :focus-visible{outline:2px solid var(--green);outline-offset:3px}
</style>
