<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useRoute, navigateTo, useHead } from '#imports'
import AuthVisualAside from '~/components/auth/AuthVisualAside.vue'
import LoginForm from '~/components/auth/LoginForm.vue'
import RegisterForm from '~/components/auth/RegisterForm.vue'
import RecoverForm from '~/components/auth/RecoverForm.vue'

definePageMeta({
  layout: 'auth'
})

const route = useRoute()

const currentMode = computed<'login' | 'register' | 'recover'>(() => {
  const mode = route.query.mode
  if (mode === 'register' || mode === 'recover') {
    return mode
  }
  return 'login'
})

const sharedEmail = ref(typeof route.query.email === 'string' ? route.query.email : '')

const titles: Record<'login' | 'register' | 'recover', string> = {
  login: '登录 — AttackBox',
  register: '注册 — AttackBox',
  recover: '找回密码 — AttackBox'
}

const descriptions: Record<'login' | 'register' | 'recover', string> = {
  login: '登录 AttackBox，继续你的安全学习与实践。',
  register: '创建 AttackBox 账号，开启你的安全学习之旅。',
  recover: '找回你的 AttackBox 账号密码。'
}

useHead({
  title: computed(() => titles[currentMode.value] || '登录 — AttackBox'),
  meta: [
    {
      name: 'description',
      content: computed(() => descriptions[currentMode.value] || '登录 AttackBox，继续你的安全学习与实践。')
    }
  ]
})

function handleSwitchMode(targetMode: 'login' | 'register' | 'recover') {
  navigateTo(
    {
      query: {
        ...route.query,
        mode: targetMode
      }
    },
    { replace: true }
  )
}

// Reset focus to email input or form title when mode switches
watch(currentMode, () => {
  if (typeof document !== 'undefined') {
    nextTick(() => {
      const emailInput = document.getElementById('email')
      if (emailInput) {
        emailInput.focus()
      } else {
        const formTitle = document.getElementById('form-title')
        formTitle?.focus()
      }
    })
  }
})
</script>

<template>
  <main class="layout">
    <!-- 左侧倾斜终端视觉画板 -->
    <AuthVisualAside />

    <!-- 右侧表单操作区 -->
    <section class="form-area" aria-labelledby="form-title">
      <LoginForm
        v-if="currentMode === 'login'"
        v-model:model-value-email="sharedEmail"
        @switch-mode="handleSwitchMode"
      />

      <RegisterForm
        v-else-if="currentMode === 'register'"
        v-model:model-value-email="sharedEmail"
        @switch-mode="handleSwitchMode"
      />

      <RecoverForm
        v-else-if="currentMode === 'recover'"
        v-model:model-value-email="sharedEmail"
        @switch-mode="handleSwitchMode"
      />
    </section>
  </main>
</template>

<style scoped>
.layout {
  flex: 1;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 65px;
  align-items: center;
  padding: 15px 0 35px;
}

.form-area {
  width: 100%;
  max-width: 360px;
  margin: auto;
  padding: 25px 0;
}

@media (min-width: 1500px) {
  .layout {
    gap: 100px;
  }
}

@media (max-width: 1100px) {
  .layout {
    gap: 40px;
    grid-template-columns: 0.95fr 1fr;
  }

  .form-area {
    max-width: 330px;
  }
}

@media (max-width: 760px) {
  .layout {
    display: block;
    flex: 1;
    padding: 0;
  }

  .form-area {
    max-width: 360px;
    padding: 60px 0 55px;
  }
}

@media (max-height: 700px) and (min-width: 761px) {
  .layout {
    padding: 10px 0 20px;
  }

  .form-area {
    padding: 10px 0;
  }
}
</style>
