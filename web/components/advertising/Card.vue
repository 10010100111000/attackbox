<script setup lang="ts">
import type { AdCreative } from '~/shared/advertising'
const props = defineProps<{ creative: AdCreative }>()
defineEmits<{ activate: [] }>()
const failed = ref(false)
watch(
  () => props.creative.imageUrl,
  () => {
    failed.value = false
  }
)
</script>

<template>
  <aside class="sponsored-card" aria-label="赞助内容">
    <div v-if="creative.imageUrl" class="sponsor-media" aria-hidden="true">
      <img v-if="!failed" :src="creative.imageUrl" alt="" loading="lazy" @error="failed = true" />
    </div>
    <div class="sponsor-copy">
      <span class="sponsor-label">{{
        creative.sponsor ? `赞助 · ${creative.sponsor}` : '广告'
      }}</span>
      <h2>{{ creative.title }}</h2>
      <p v-if="creative.description">{{ creative.description }}</p>
    </div>
    <UiButton
      :href="creative.href"
      as="a"
      variant="secondary"
      size="sm"
      target="_blank"
      rel="sponsored noopener noreferrer"
      class="sponsor-link"
      @click="$emit('activate')"
      >{{ creative.cta || '了解更多' }}</UiButton
    >
  </aside>
</template>

<style scoped>
.sponsored-card {
  display: flex;
  align-items: center;
  gap: 22px;
  padding: 22px;
  border: 1px solid var(--line);
  border-radius: 12px;
  background: var(--paper);
  color: var(--ink);
}
.sponsor-media {
  width: 160px;
  height: 96px;
  overflow: hidden;
  border-radius: 7px;
  flex-shrink: 0;
  background: var(--soft);
}
.sponsor-media img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.sponsor-copy {
  min-width: 0;
  flex: 1;
}
.sponsor-label {
  display: block;
  margin-bottom: 6px;
  font-size: 10px;
  color: var(--muted);
}
h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 650;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
p {
  margin: 7px 0 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--muted);
  overflow-wrap: anywhere;
}
.sponsor-copy h2,
.sponsor-copy p {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
}
.sponsor-link {
  flex-shrink: 0;
}
@media (max-width: 640px) {
  .sponsored-card {
    flex-wrap: wrap;
    gap: 16px;
    padding: 18px;
  }
  .sponsor-media {
    width: 96px;
    height: 72px;
  }
  .sponsor-copy {
    min-width: 150px;
  }
  .sponsor-link {
    margin-left: auto;
  }
}
</style>
