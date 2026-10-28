<script setup lang="ts">
const props = defineProps<{ endpoint: string }>()
const emit = defineEmits<{ saved: [] }>()
const confirmAction = useConfirm()
const { data, pending, error, refresh } = await useLazyFetch<{data:{repository:string;branch:string;directory:string;hasToken:boolean;version:number}}>(() => props.endpoint)
const form = ref({repository:'',branch:'main',directory:'',token:''}), busy = ref(false), notice = ref('')
watch(data, value => { if (value) form.value={repository:value.data.repository,branch:value.data.branch,directory:value.data.directory,token:''} }, {immediate:true})
async function save(clearToken = false) {
  if (busy.value || !data.value) return
  if (clearToken && !(await confirmAction({title:'清除资料仓库 Token？',description:'已导入资料不受影响，私有仓库再次导入前需重新设置凭据。',confirmLabel:'清除 Token'}))) return
  busy.value=true;notice.value=''
  try {
    const source=clearToken?{...data.value.data,token:'',clearToken:true}:form.value
    await $fetch(props.endpoint,{method:'PUT',body:{...source,version:data.value.data.version},retry:false})
    form.value.token='';await refresh();emit('saved')
  } catch(e:any) { notice.value=e?.data?.message||'配置保存失败，输入已保留' }
  finally { busy.value=false }
}
</script>
<template>
  <section class="panel repository-settings" aria-label="资料仓库设置">
    <h2>GitHub 仓库</h2><UiBusyIndicator v-if="pending&&!data" label="仓库设置加载"/>
    <p v-if="error||notice" class="notice" role="alert">{{notice||'无法读取仓库设置'}}</p>
    <form v-if="data" @submit.prevent="save()">
      <div class="repo-fields"><label>仓库<input v-model="form.repository" required maxlength="240" placeholder="owner/repository"/></label><label>分支<input v-model="form.branch" required maxlength="200"/></label><label>资料目录<input v-model="form.directory" maxlength="500" placeholder="references"/></label></div>
      <label class="token-field">只读 Token<input v-model="form.token" type="password" autocomplete="new-password" maxlength="4096" :placeholder="data.data.hasToken?'已设置，留空保留':'公开仓库可留空'"/></label>
      <div class="repo-footer"><p class="muted">与教程使用独立扫描目录。Token 加密保存，不回显。</p><div class="actions"><button v-if="data.data.hasToken" type="button" :disabled="busy" @click="save(true)">清除 Token</button><button type="submit" class="primary" :disabled="busy||!form.repository.trim()"><UiBusyLabel :busy="busy">保存仓库</UiBusyLabel></button></div></div>
    </form>
  </section>
</template>
<style scoped>
.repo-fields { display:grid;grid-template-columns:minmax(260px,2fr) minmax(120px,.7fr) minmax(180px,1fr);gap:20px; }
label { display:grid;gap:9px;font-size:12px;color:var(--muted); }
input { width:100%;height:38px;padding:8px 11px;border:1px solid var(--line);border-radius:6px;background:var(--soft);color:var(--ink);font-size:13px; }
.token-field { max-width:460px;margin-top:20px; }
.repo-footer { display:flex;justify-content:space-between;align-items:end;gap:24px;margin-top:24px; }.repo-footer p{margin:0}
</style>
