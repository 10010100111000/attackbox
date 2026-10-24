<script setup lang="ts">
import {ref,computed} from 'vue'
import {useRoute} from '#imports'
import {useAuth} from '~/composables/useAuth'
import Button from '~/components/ui/Button.vue'
import Input from '~/components/ui/Input.vue'
const emit=defineEmits<{'switch-mode':[mode:'login'|'recover']}>()
const route=useRoute(),{resetPassword}=useAuth()
const token=computed(()=>typeof route.query.token==='string'?route.query.token:'')
const password=ref(''),confirmation=ref(''),busy=ref(false),done=ref(false),error=ref('')
async function submit(){
 if(busy.value)return
 error.value=''
 if(!token.value){error.value='链接缺少重置凭证，请重新申请。';return}
 if(password.value.length<6){error.value='新密码至少需要 6 位。';return}
 if(password.value!==confirmation.value){error.value='两次输入的密码不一致。';return}
 busy.value=true
 try{await resetPassword(token.value,password.value);done.value=true;password.value='';confirmation.value=''}
 catch(e:any){error.value=e?.data?.message||e?.data?.statusMessage||'重设失败，请重新申请重置链接。'}
 finally{busy.value=false}
}
</script>
<template><div class="reset-form"><h1>重设密码</h1><template v-if="done"><p role="status">密码已更新，请重新登录。</p><Button @click="emit('switch-mode','login')">返回登录</Button></template><form v-else @submit.prevent="submit"><p>为你的账号设置新密码。</p><label for="reset-password">新密码</label><Input id="reset-password" v-model="password" type="password" autocomplete="new-password" required :disabled="busy"/><label for="reset-confirm">确认密码</label><Input id="reset-confirm" v-model="confirmation" type="password" autocomplete="new-password" required :disabled="busy"/><p v-if="error" role="alert">{{error}}</p><Button type="submit" :disabled="busy">{{busy?'正在保存…':'更新密码'}}</Button><button type="button" :disabled="busy" @click="emit('switch-mode','recover')">重新申请重置链接</button></form></div></template>
<style scoped>.reset-form{width:100%}h1{font-size:28px;margin:0 0 18px}p{color:var(--muted);line-height:1.8;font-size:14px}form{display:grid;gap:16px}label{font-size:13px}button[type=button]{background:none;border:0;color:var(--muted);cursor:pointer;padding:10px}p[role=alert]{color:var(--error,#c65b5b)}</style>
