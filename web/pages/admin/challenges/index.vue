<script setup lang="ts">
import {Plus} from 'lucide-vue-next'
import type {ChallengeSummary} from '~/shared/challenges'
definePageMeta({layout:'admin'})
useHead({title:'挑战管理 — AttackBox'})
const {data,error,refresh}=await useFetch<{data:ChallengeSummary[]}>('/api/admin/challenges',{key:'admin:challenges'})
const query=ref(''),notice=ref('')
const rows=computed(()=>data.value?.data.filter(c=>`${c.title} ${c.category}`.toLowerCase().includes(query.value.toLowerCase()))||[])
async function remove(c:ChallengeSummary){if(!confirm(`删除“${c.title}”及阶段完成记录？`))return;try{await $fetch(`/api/admin/challenges/${c.id}`,{method:'DELETE',body:{lockVersion:c.lockVersion}});await refresh()}catch(e:any){notice.value=e?.data?.message||'删除失败'}}
</script>
<template><div class="users-page"><header class="page-heading"><div><h1>挑战管理</h1><p>维护挑战说明、阶段 Flag 和关联靶机模板。</p></div><NuxtLink to="/admin/challenges/new"><Plus :size="15"/>新建挑战</NuxtLink></header><input v-model="query" aria-label="搜索挑战" placeholder="搜索名称或分类"/><p v-if="error||notice" class="notice" role="alert">{{notice||'读取失败，请刷新重试'}}</p><div class="panel table-wrap"><table><thead><tr><th>挑战</th><th>分类</th><th>阶段</th><th>状态</th><th class="actions">操作</th></tr></thead><tbody><tr v-for="c in rows" :key="c.id"><td><NuxtLink :to="`/admin/challenges/${c.id}`" class="user-name">{{c.title}}</NuxtLink><span class="secondary">{{c.summary}}</span></td><td>{{c.category||'未分类'}}</td><td>{{c.totalStages}}</td><td>{{c.status==='published'?'已发布':'草稿'}}</td><td class="actions"><NuxtLink :to="`/admin/challenges/${c.id}`">编辑</NuxtLink><button @click="remove(c)">删除</button></td></tr></tbody></table><p v-if="!rows.length&&!error" class="empty">暂无挑战</p></div></div></template>
<style scoped src="~/assets/css/admin-users.css"></style>
<style scoped>input{border:1px solid var(--line);border-radius:6px;padding:10px;margin-bottom:20px;background:var(--paper)}td button{margin-left:16px}</style>
