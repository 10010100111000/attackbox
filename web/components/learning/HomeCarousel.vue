<script setup lang="ts">
import type { HomeSlide } from '~/shared/learn-home'
import { Pause, Play } from 'lucide-vue-next'
const props = withDefaults(
  defineProps<{ slides: HomeSlide[]; autoplay?: boolean; intervalSeconds?: number }>(),
  { autoplay: true, intervalSeconds: 6 }
)
const failedImages = ref(new Set<string>())
const index = ref(0),
  hovered = ref(false),
  focused = ref(false),
  paused = ref(false),
  hidden = ref(false),
  reduced = ref(false)
const slide = computed(() => props.slides[index.value])
const hasImage = computed(
  () => !!slide.value?.imageUrl && !failedImages.value.has(slide.value.imageUrl)
)
let mounted = false,
  timer: ReturnType<typeof setInterval> | undefined,
  media: MediaQueryList | undefined
function syncTimer() {
  clearInterval(timer)
  timer = undefined
  if (
    mounted &&
    props.autoplay &&
    props.slides.length > 1 &&
    !hovered.value &&
    !focused.value &&
    !paused.value &&
    !hidden.value &&
    !reduced.value
  )
    timer = setInterval(() => {
      index.value = (index.value + 1) % props.slides.length
    }, props.intervalSeconds * 1000)
}
function select(i: number) {
  index.value = i
  syncTimer()
}
function focusIn(e: FocusEvent) {
  // Pointer clicks leave DOM focus behind; only keyboard focus pauses playback.
  focused.value = e.target instanceof Element && e.target.matches(':focus-visible')
}
function togglePlayback() {
  paused.value = !paused.value
  if (!paused.value) focused.value = false
}
function focusOut(e: FocusEvent) {
  if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node | null))
    focused.value = false
}
function visibility() {
  hidden.value = document.hidden
}
function motion() {
  reduced.value = !!media?.matches
}
watch(
  () => props.slides,
  () => {
    index.value = 0
    failedImages.value = new Set()
    syncTimer()
  }
)
watch(
  [hovered, focused, paused, hidden, reduced, () => props.autoplay, () => props.intervalSeconds],
  syncTimer
)
onMounted(() => {
  mounted = true
  media = window.matchMedia('(prefers-reduced-motion: reduce)')
  motion()
  visibility()
  media.addEventListener('change', motion)
  document.addEventListener('visibilitychange', visibility)
  syncTimer()
})
onUnmounted(() => {
  mounted = false
  clearInterval(timer)
  media?.removeEventListener('change', motion)
  document.removeEventListener('visibilitychange', visibility)
})
</script>
<template>
  <section
    v-if="slide"
    class="feature-banner"
    :class="{ 'has-image': hasImage, 'has-pagination': slides.length > 1 }"
    aria-roledescription="轮播"
    aria-label="推荐内容"
    @mouseenter="hovered = true"
    @mouseleave="hovered = false"
    @pointerdown="focused = false"
    @focusin="focusIn"
    @focusout="focusOut"
    @keydown.left.prevent="select((index - 1 + slides.length) % slides.length)"
    @keydown.right.prevent="select((index + 1) % slides.length)"
  >
    <LearningBannerBackdrop v-if="!hasImage" :seed="slide.id" />
    <img
      v-if="hasImage"
      :key="slide.imageUrl"
      :src="slide.imageUrl"
      alt=""
      class="banner-image"
      @error="failedImages.add(slide.imageUrl)"
    />
    <div class="carousel-copy" :aria-live="paused || !autoplay ? 'polite' : 'off'">
      <div v-if="slide.label" class="feature-label">{{ slide.label }}</div>
      <h1>{{ slide.title }}</h1>
      <p v-if="slide.description">{{ slide.description }}</p>
      <UiButton
        v-if="slide.href"
        :to="slide.href"
        variant="secondary"
        size="sm"
        class="banner-link"
        >{{ slide.cta || '了解更多' }}</UiButton
      >
    </div>
    <div v-if="slides.length > 1" class="carousel-pagination" aria-label="选择推荐内容">
      <button
        v-for="(s, i) in slides"
        :key="s.id"
        type="button"
        class="carousel-dot"
        :class="{ active: index === i }"
        :aria-label="`第 ${i + 1} 项：${s.title}`"
        :aria-current="index === i ? 'true' : undefined"
        @click="select(i)"
      >
        <span />
      </button>
      <button
        v-if="autoplay && !reduced"
        type="button"
        class="carousel-dot"
        :aria-label="paused ? '播放轮播' : '暂停轮播'"
        :aria-pressed="paused"
        @click="togglePlayback"
      >
        <Play v-if="paused" :size="12" /><Pause v-else :size="12" />
      </button>
    </div>
  </section>
</template>
<style scoped>
.feature-banner {
  --banner-ink: var(--ink);
  --banner-muted: var(--muted);
  position: relative;
  isolation: isolate;
  overflow: hidden;
  min-height: 176px;
  padding: 22px 28px;
  border: 1px solid var(--line);
  border-radius: 12px;
  margin-bottom: 18px;
  background: linear-gradient(120deg, var(--soft), var(--paper));
  box-shadow: var(--surface-highlight), var(--card-shadow);
  display: flex;
  align-items: center;
}
.has-pagination {
  padding-bottom: 44px;
}
.has-image {
  --banner-ink: #fff;
  --banner-muted: rgba(255, 255, 255, 0.8);
}
.banner-image {
  position: absolute;
  inset: 0;
  z-index: -2;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.has-image::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  background: linear-gradient(
    90deg,
    rgba(7, 10, 16, 0.88),
    rgba(7, 10, 16, 0.64) 55%,
    rgba(7, 10, 16, 0.24)
  );
  pointer-events: none;
}
.carousel-copy {
  min-width: 0;
  width: min(680px, 80%);
  overflow-wrap: anywhere;
}
.feature-label {
  margin-bottom: 8px;
  color: var(--banner-muted);
  font-size: 11px;
  line-height: 1.4;
}
.feature-banner h1 {
  margin: 0;
  font-size: 23px;
  font-weight: 650;
  letter-spacing: -0.6px;
  line-height: 1.35;
  color: var(--banner-ink);
}
.feature-banner p {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--banner-muted);
}
.banner-link {
  margin-top: 14px;
}
.feature-banner h1,
.feature-banner p {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
}
.carousel-pagination {
  position: absolute;
  bottom: 10px;
  left: 24px;
  display: flex;
  align-items: center;
  gap: 4px;
}
.carousel-dot {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  padding: 8px 4px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  cursor: pointer;
  color: var(--banner-ink);
}
.carousel-dot span {
  width: 6px;
  height: 4px;
  border-radius: 5px;
  background: currentColor;
  opacity: 0.4;
  transition:
    width 0.2s ease,
    opacity 0.2s ease;
}
.carousel-dot.active span {
  width: 18px;
  opacity: 1;
}
.carousel-dot:focus-visible {
  outline: 2px solid var(--banner-ink);
  outline-offset: 2px;
}
@media (max-width: 640px) {
  .feature-banner {
    padding-inline: 22px;
  }
  .carousel-copy {
    width: 100%;
  }
  .feature-banner h1 {
    font-size: 21px;
  }
  .has-image::before {
    background: rgba(7, 10, 16, 0.76);
  }
}
@media (prefers-reduced-motion: reduce) {
  .carousel-dot span {
    transition: none;
  }
}
</style>
