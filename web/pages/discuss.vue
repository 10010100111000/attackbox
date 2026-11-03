<script setup lang="ts">
import { MessageCircle, Sparkles, BookOpen, Flag } from 'lucide-vue-next'

definePageMeta({ layout: 'learning' })
useHead({
  title: '讨论 — AttackBox',
  meta: [{ name: 'robots', content: 'noindex' }]
})
</script>

<template>
  <div class="learning-frame learning-shell discuss-page">
    <section class="discuss-card">
      <div class="conversation-sketch" aria-hidden="true">
        <MessageCircle :size="72" :stroke-width="1.3" />
        <span>?</span>
        <Sparkles :size="22" class="spark" />
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
  border: 1px solid var(--line-strong, var(--line));
  border-radius: 16px;
  background:
    radial-gradient(ellipse 60% 50% at 50% 0%, var(--brand-soft), transparent 65%),
    var(--card-surface);
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
  margin-bottom: 26px;
}

.conversation-sketch > span {
  position: absolute;
  top: 15px;
  left: 26px;
  font-size: 28px;
  font-weight: 650;
}

.spark {
  position: absolute;
  right: -26px;
  top: -12px;
  color: var(--brand);
  transform: rotate(18deg);
}

.eyebrow {
  padding: 4px 10px;
  border-radius: 6px;
  background: var(--surface-well);
  border: 1px solid var(--well-border);
  color: var(--brand);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.6px;
}

h1 {
  font-size: 32px;
  font-weight: 700;
  margin: 14px 0 12px;
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
  margin-top: 30px;
}

.actions a {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 11px 18px;
  border-radius: 8px;
  text-decoration: none;
  border: 1px solid var(--line-strong, var(--line));
  background: var(--card-surface);
  box-shadow: var(--surface-highlight);
  color: var(--ink);
  font-size: 13px;
  font-weight: 600;
  transition:
    transform 0.16s ease,
    border-color 0.16s ease,
    background 0.16s ease;
}

.actions a:first-child {
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  border-color: transparent;
  box-shadow: var(--btn-primary-shadow);
}

.actions a:hover {
  transform: translateY(-2px);
  border-color: var(--brand-border);
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
