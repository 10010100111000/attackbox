import { watch, onMounted, onBeforeUnmount, type Ref } from 'vue'

/** One qualification per document: 30 visible seconds and 25% reading depth.
 * Consumers pass their actual scroller; shared reader components stay independent. */
export function useReadingAnalytics(container: Readonly<Ref<HTMLElement | null>>) {
  const route = useRoute(),
    nuxtApp = useNuxtApp()
  let element: HTMLElement | null = null,
    path = '',
    mounted = false,
    reported = false
  let elapsed = 0,
    depth = 0,
    since: number | null = null,
    timer: ReturnType<typeof setTimeout> | undefined
  function check() {
    if (!element || reported) return
    const distance = Math.max(0, element.scrollHeight - element.clientHeight)
    depth = Math.max(depth, distance ? Math.min(1, element.scrollTop / distance) : 1)
    const visibleTime = elapsed + (since !== null ? performance.now() - since : 0)
    if (visibleTime >= 30000 && depth >= 0.25) {
      reported = true
      clearTimeout(timer)
      void nuxtApp.$siteAnalytics?.read(path)
    }
  }
  function visibility() {
    clearTimeout(timer)
    if (since !== null) elapsed += performance.now() - since
    since = document.hidden ? null : performance.now()
    if (since !== null && !reported && elapsed < 30000) timer = setTimeout(check, 30000 - elapsed)
    check()
  }
  function observe() {
    if (!mounted) return
    element?.removeEventListener('scroll', check)
    clearTimeout(timer)
    element = container.value
    path = route.path
    elapsed = 0
    depth = 0
    reported = false
    since = null
    element?.addEventListener('scroll', check, { passive: true })
    visibility()
  }
  watch([container, () => route.path], observe, { flush: 'post' })
  onMounted(() => {
    mounted = true
    observe()
    document.addEventListener('visibilitychange', visibility)
  })
  onBeforeUnmount(() => {
    clearTimeout(timer)
    element?.removeEventListener('scroll', check)
    document.removeEventListener('visibilitychange', visibility)
  })
}
