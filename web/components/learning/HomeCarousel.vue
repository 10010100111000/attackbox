<script setup lang="ts">
import type { HomeSlide } from '~/shared/learn-home'
import { Pause, Play } from 'lucide-vue-next'
const props = defineProps<{slides: HomeSlide[]; autoplay: boolean; intervalSeconds: number}>()
const index = ref(0), hovered = ref(false), focused = ref(false), paused = ref(false), hidden = ref(false), reduced = ref(false)
const slide = computed(() => props.slides[index.value])
let mounted = false, timer: ReturnType<typeof setInterval> | undefined, media: MediaQueryList | undefined
function syncTimer() {
  clearInterval(timer); timer = undefined
  if (mounted && props.autoplay && props.slides.length > 1 && !hovered.value && !focused.value && !paused.value && !hidden.value && !reduced.value)
    timer = setInterval(() => { index.value = (index.value + 1) % props.slides.length }, props.intervalSeconds * 1000)
}
function select(i: number) { index.value = i; syncTimer() }
function focusOut(e: FocusEvent) { if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node | null)) focused.value = false }
function visibility() { hidden.value = document.hidden }
function motion() { reduced.value = !!media?.matches }
watch(() => props.slides, () => { index.value = 0; syncTimer() })
watch([hovered, focused, paused, hidden, reduced, () => props.autoplay, () => props.intervalSeconds], syncTimer)
onMounted(() => {
  mounted = true; media = window.matchMedia('(prefers-reduced-motion: reduce)'); motion(); visibility()
  media.addEventListener('change', motion); document.addEventListener('visibilitychange', visibility); syncTimer()
})
onUnmounted(() => { mounted = false; clearInterval(timer); media?.removeEventListener('change', motion); document.removeEventListener('visibilitychange', visibility) })
</script>
<template>
 <section v-if="slide" class="feature-banner" aria-roledescription="轮播" aria-label="推荐内容" @mouseenter="hovered=true" @mouseleave="hovered=false" @focusin="focused=true" @focusout="focusOut" @keydown.left.prevent="select((index-1+slides.length)%slides.length)" @keydown.right.prevent="select((index+1)%slides.length)">
  <div class="carousel-copy" :aria-live="paused || !autoplay ? 'polite' : 'off'">
   <div class="feature-label">{{ slide.label }}</div><h1>{{ slide.title }}</h1><p>{{ slide.description }}</p><NuxtLink :to="slide.href" class="btn">{{ slide.cta }}</NuxtLink>
  </div>
  <div class="banner-art" aria-hidden="true"><div class="terminal-art"><div class="dots"><i/><i/><i/></div><div class="cmd">{{ slide.command }}</div><div class="dim">{{ slide.caption }}</div><div class="cursor-line">▍</div></div></div>
  <div v-if="slides.length>1" class="carousel-pagination" aria-label="选择推荐内容">
   <button v-for="(s,i) in slides" :key="s.id" class="carousel-dot" :class="{active:index===i}" :aria-label="`第 ${i+1} 项：${s.title}`" :aria-current="index===i ? 'true' : undefined" @click="select(i)"><span/></button>
   <button v-if="autoplay && !reduced" class="carousel-dot" :aria-label="paused ? '播放轮播' : '暂停轮播'" :aria-pressed="paused" @click="paused=!paused"><Play v-if="paused" :size="12"/><Pause v-else :size="12"/></button>
  </div>
 </section>
</template>
<style scoped>
/* 轮播 Banner (完全匹配原型，移除了验证性的 controls) */
.feature-banner {
  display: grid;
  grid-template-columns: 1fr 230px;
  gap: 25px;
  align-items: center;
  padding: 22px 28px;
  border: 1px solid color-mix(in srgb, var(--brand) 22%, var(--line));
  background: radial-gradient(circle at 88% 30%, var(--brand-soft), transparent 52%), var(--paper);
  box-shadow: var(--surface-highlight), var(--card-shadow);
  border-radius: 12px;
  min-height: 174px;
  position: relative;
  overflow: hidden;
  margin-bottom: 18px;
}

.carousel-copy {
  min-width: 0;
  z-index: 2;
}

.feature-label {
  color: var(--muted);
  font-size: 11px;
  margin-bottom: 10px;
  display: flex;
  align-items: center;
  gap: 9px;
  letter-spacing: 0.5px;
}

.feature-label::before {
  content: '';
  display: block;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--green);
}

.feature-banner h1 {
  font-size: 24px;
  margin: 0 0 8px;
  letter-spacing: -0.8px;
  color: var(--ink);
  line-height: 1.35;
}

.feature-banner p {
  font-size: 13px;
  margin: 0 0 18px;
  max-width: 540px;
  color: var(--muted);
  line-height: 1.6;
}

.feature-banner .btn {
  font-size: 12px;
  padding: 9px 18px;
  border-radius: 6px;
  background: var(--green);
  color: var(--on-accent);
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  font-weight: 550;
  transition: opacity 0.15s ease;
}

.feature-banner .btn:hover {
  opacity: 0.9;
}

/* 轮播右侧倾斜终端艺术卡片 */
.banner-art {
  height: 110px;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
}

.terminal-art {
  width: 172px;
  background: var(--code);
  color: var(--code-ink);
  border-radius: 8px;
  padding: 12px 16px;
  transform: rotate(-4deg);
  box-shadow: 9px 10px 0 var(--soft);
  font: 12px/2 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.terminal-art .dots {
  display: flex;
  gap: 4px;
  margin-bottom: 12px;
}

.terminal-art .dots i {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--code-muted);
  opacity: 0.65;
}

.terminal-art .cmd {
  color: var(--code-highlight);
}

.terminal-art .dim {
  color: var(--code-muted);
  font-size: 10px;
}

.cursor-line {
  margin-top: 5px;
  color: var(--green);
  animation: blink 1s step-start infinite;
}

@keyframes blink {
  50% { opacity: 0; }
}

/* 轮播分页胶囊指示器 (仅保留纯粹的分页点) */
.carousel-pagination {
  position: absolute;
  bottom: 14px;
  left: 28px;
  display: flex;
  align-items: center;
  gap: 4px;
  z-index: 5;
}

.carousel-dot {
  width: 26px;
  height: 26px;
  padding: 8px 4px;
  background: transparent;
  display: grid;
  place-items: center;
  border-radius: 5px;
  border: none;
  cursor: pointer;
}

.carousel-dot span {
  height: 4px;
  width: 6px;
  border-radius: 5px;
  background: var(--muted);
  opacity: 0.35;
  transition: width 0.2s ease, opacity 0.2s ease, background-color 0.2s ease;
}

.carousel-dot.active span {
  width: 18px;
  opacity: 1;
  background: var(--ink);
}


.carousel-copy {overflow-wrap:anywhere}.feature-banner h1,.feature-banner p{display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:2;overflow:hidden}.terminal-art {overflow-wrap:anywhere}.carousel-dot{color:var(--muted)}
button:focus-visible,a:focus-visible{outline:2px solid var(--green);outline-offset:3px}
@media(max-width:640px){.feature-banner{grid-template-columns:1fr;padding:24px 22px 46px}.banner-art{display:none}.feature-banner h1{font-size:23px}}
@media(prefers-reduced-motion:reduce){.cursor-line{animation:none}.carousel-dot span{transition:none}}
</style>
