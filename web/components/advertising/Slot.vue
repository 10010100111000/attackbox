<script setup lang="ts">
import type { AdCreative, AdPlacement } from '~/shared/advertising'
const props = defineProps<{ placement: AdPlacement }>()
const route = useRoute(),
  nuxtApp = useNuxtApp()
const { data } = await useFetch<{ data: AdCreative | null }>(() => `/api/ads/${props.placement}`, {
  key: `advertising:${props.placement}:${route.path}`,
  timeout: 1500,
  retry: false
})
const element = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | undefined, timer: ReturnType<typeof setTimeout> | undefined
let visible = false,
  impressed = false
function clear() {
  clearTimeout(timer)
  timer = undefined
}
function schedule() {
  clear()
  if (!visible || document.hidden || impressed || !data.value?.data) return
  timer = setTimeout(() => {
    if (!visible || document.hidden || !data.value?.data) return
    impressed = true
    void nuxtApp.$siteAnalytics?.advertisement(
      route.path,
      data.value.data.id,
      props.placement,
      'impression'
    )
  }, 1000)
}
function activate() {
  if (!data.value?.data) return
  impressed = true
  clear()
  void nuxtApp.$siteAnalytics?.advertisement(
    route.path,
    data.value.data.id,
    props.placement,
    'click'
  )
}
onMounted(() => {
  if (!element.value) return
  observer = new IntersectionObserver(
    entries => {
      visible = !!entries[0]?.isIntersecting && entries[0].intersectionRatio >= 0.5
      schedule()
    },
    { threshold: [0.5] }
  )
  observer.observe(element.value)
  document.addEventListener('visibilitychange', schedule)
})
onBeforeUnmount(() => {
  clear()
  observer?.disconnect()
  document.removeEventListener('visibilitychange', schedule)
})
</script>

<template>
  <div v-if="data?.data" ref="element" class="ad-slot" :data-placement="placement">
    <AdvertisingCard :creative="data.data" @activate="activate" />
  </div>
</template>

<style scoped>
.ad-slot {
  margin-block: 28px;
}
</style>
