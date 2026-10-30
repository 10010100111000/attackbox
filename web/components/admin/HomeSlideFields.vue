<script setup lang="ts">
import type { HomeSlide } from '~/shared/learn-home'
const slide = defineModel<HomeSlide>({ required: true })
</script>

<template>
  <div class="fields">
    <label class="wide"
      >标题 *<UiInput v-model="slide.title" bare maxlength="100" required
    /></label>
    <label class="wide"
      >图片地址<UiInput
        v-model="slide.imageUrl"
        bare
        maxlength="1024"
        inputmode="url"
        placeholder="https://cdn.example.com/banner.webp"
      />
      <small>HTTPS 图片地址或本站路径，建议使用横向图片。</small>
    </label>
    <label class="wide"
      >宣传文案<UiTextarea v-model="slide.description" maxlength="300" rows="2"
    /></label>
    <div class="link-fields wide">
      <label
        >跳转链接<UiInput
          v-model="slide.href"
          bare
          list="home-destinations"
          maxlength="1024"
          placeholder="/learn 或 https://example.com/campaign"
      /></label>
      <label
        >按钮文字<UiInput v-model="slide.cta" bare maxlength="30" placeholder="了解更多"
      /></label>
    </div>
    <label
      >标签<UiInput
        v-model="slide.label"
        bare
        maxlength="30"
        placeholder="例如：本期精选、活动、广告"
    /></label>
  </div>
</template>

<style scoped>
.fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
}

.fields label {
  display: flex;
  flex-direction: column;
  gap: 7px;
  font-size: 13px;
}

.link-fields {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 180px;
  gap: 18px;
}

.wide {
  grid-column: 1 / -1;
}
small {
  color: var(--muted);
  font-size: 12px;
  font-weight: 400;
  line-height: 1.6;
}
</style>
