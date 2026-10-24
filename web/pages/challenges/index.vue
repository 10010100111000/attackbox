<script setup lang="ts">
import {Flag,ArrowRight,CheckCircle2} from 'lucide-vue-next'
import {useAuth} from '~/composables/useAuth'
import type {ChallengeSummary} from '~/shared/challenges'
definePageMeta({layout:'learning'})
useHead({title:'挑战 — AttackBox'})
const {currentUser}=useAuth(),learner=computed(()=>currentUser.value?.id||'')
const {data,error,refresh}=await useFetch<{data:ChallengeSummary[]}>('/api/challenges',{key:'challenges:list',watch:[learner]})
const query=ref(''),category=ref(''),difficulty=ref('')
const categories=computed(()=>[...new Set(data.value?.data.map(c=>c.category).filter(Boolean)||[])])
const rows=computed(()=>data.value?.data.filter(c=>(!category.value||c.category===category.value)&&(!difficulty.value||c.difficulty===difficulty.value)&&c.title.toLowerCase().includes(query.value.toLowerCase()))||[])
const labels:Record<string,string>={beginner:'入门',intermediate:'进阶',advanced:'高级'}
</script>
<template><main class="challenge-list"><header><Flag :size="22"/><h1>挑战</h1></header><p class="intro">从一个目标开始，逐步找到答案。</p><div class="filters"><input v-model="query" aria-label="搜索挑战" placeholder="搜索挑战"/><select v-model="category" aria-label="挑战分类"><option value="">全部分类</option><option v-for="c in categories" :key="c">{{c}}</option></select><select v-model="difficulty" aria-label="挑战难度"><option value="">全部难度</option><option v-for="(label,key) in labels" :key="key" :value="key">{{label}}</option></select></div><p v-if="error" role="alert">无法读取挑战。<button @click="refresh()">重试</button></p><div v-else class="cards"><NuxtLink v-for="c in rows" :key="c.id" :to="`/challenges/${c.id}`" class="card"><div class="meta"><span>{{c.category||'综合'}} · {{labels[c.difficulty]}}</span><CheckCircle2 v-if="currentUser&&c.totalStages>0&&c.completedStages===c.totalStages" :size="17"/></div><h2>{{c.title}}</h2><p>{{c.summary}}</p><footer><span>{{currentUser?`${c.completedStages} / ${c.totalStages} 阶段已完成`:`${c.totalStages} 个阶段`}}</span><ArrowRight :size="16"/></footer></NuxtLink></div><p v-if="!error&&!rows.length" class="empty">暂无符合条件的挑战。</p></main></template>
<style scoped>
.challenge-list{width:100%;max-width:1140px;margin:auto;padding:44px 28px 80px}header{display:flex;gap:12px;align-items:center}h1{font-size:30px;margin:0}.intro{color:var(--muted);font-size:14px;margin:15px 0 30px}.filters{display:flex;gap:12px;margin-bottom:26px}input,select,button{padding:10px 12px;background:var(--paper);color:var(--ink);border:1px solid var(--line);border-radius:6px;font-size:13px}input{max-width:300px;flex:1;min-width:100px}.cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:20px}.card{background:var(--paper);border:1px solid var(--line);border-radius:10px;padding:26px;text-decoration:none;color:var(--ink)}.meta,footer{display:flex;align-items:center;justify-content:space-between;color:var(--muted);font-size:12px}.meta svg{color:var(--green)}h2{font-size:21px;margin:20px 0 12px}.card p{font-size:13px;color:var(--muted);line-height:1.7;min-height:44px}footer{border-top:1px solid var(--line);padding-top:18px;margin-top:24px}.empty{padding:60px;text-align:center;color:var(--muted)}
</style>
