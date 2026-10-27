<script setup lang="ts">
import { MessageCircle, Sparkles, BookOpen, Flag } from 'lucide-vue-next'

definePageMeta({ layout: 'learning' })
useHead({
  title: '讨论 — AttackBox',
  meta: [{ name: 'robots', content: 'noindex' }],
})
</script>

<template>
  <div class="learning-shell discuss-page">
    <section class="discuss-card">
      <div class="conversation-sketch" aria-hidden="true">
        <MessageCircle :size="76" :stroke-width="1.3" />
        <span>?</span>
        <Sparkles :size="23" class="spark" />
      </div>
      <span class="eyebrow">讨论 · 暂未开放</span>
      <h1>好问题，值得聊聊。</h1>
      <p>
        这里会留给漏洞研究、学习交流与实践分享。<br />
        讨论功能尚未上线，先去动手试试。
      </p>
      <div class="actions">
        <NuxtLink to="/learn">
          <BookOpen :size="16" />
          继续学习
        </NuxtLink>
        <NuxtLink to="/challenges">
          <Flag :size="16" />
          看看挑战
        </NuxtLink>
      </div>
    </section>
  </div>
</template>

<style scoped>
.discuss-card {
  border: 1px solid var(--line);
  border-radius: 16px;
  background: radial-gradient(circle at 50% 0%, var(--brand-soft), transparent 60%), var(--paper);
  box-shadow: var(--surface-highlight), var(--card-shadow);
  padding: 64px 48px;
  min-height: 440px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  text-align: center;
  animation: surface-rise 0.32s cubic-bezier(0.22, 1, 0.36, 1) both;
}

.conversation-sketch {
  position: relative;
  color: var(--ink);
  transform: rotate(-7deg);
  margin-bottom: 28px;
}

.conversation-sketch > span {
  position: absolute;
  top: 16px;
  left: 27px;
  font-size: 29px;
  font-weight: 600;
}

.spark {
  position: absolute;
  right: -27px;
  top: -12px;
  color: var(--brand);
  transform: rotate(18deg);
}

.eyebrow {
  color: var(--muted);
  font-size: 12px;
  letter-spacing: 0.5px;
}

h1 {
  font-size: 30px;
  margin: 12px 0;
  letter-spacing: -1px;
}

p {
  color: var(--muted);
  font-size: 14px;
  line-height: 1.9;
  margin: 0;
}

.actions {
  display: flex;
  gap: 14px;
  margin-top: 28px;
}

.actions a {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 11px 18px;
  border-radius: 8px;
  text-decoration: none;
  border: 1px solid var(--line);
  background: var(--paper);
  color: var(--ink);
  font-size: 13px;
  font-weight: 550;
  transition: transform 0.16s ease, background 0.16s ease;
}

.actions a:first-child {
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  border-color: transparent;
  box-shadow: var(--btn-primary-shadow);
}

.actions a:hover {
  transform: translateY(-2px);
}

@media (max-width: 640px) {
  .discuss-card {
    padding: 40px 20px;
  }
  h1 {
    font-size: 25px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .actions a {
    transition: none;
  }
  .actions a:hover {
    transform: none;
  }
}
</style>
