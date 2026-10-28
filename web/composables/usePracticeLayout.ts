import { computed, ref, watch, onMounted, onBeforeUnmount, type Ref } from 'vue'

export type PracticePlacement = 'bottom' | 'right'
const storageKey = 'attackbox-reader-practice-layout'
const minReaderWidth = 480, minPracticeWidth = 420
const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value))

export function usePracticeLayout(workspace: Ref<HTMLElement | null>) {
  const preferred = ref<PracticePlacement>('right')
  const widthRatio = ref(.5), savedHeight = ref(310)
  const availableWidth = ref(0), availableHeight = ref(0)
  const sideAvailable = computed(() => availableWidth.value >= 1000)
  const placement = computed(() => sideAvailable.value ? preferred.value : 'bottom')
  const maxWidth = computed(() => Math.max(minPracticeWidth, availableWidth.value - minReaderWidth))
  const width = computed(() => clamp(Math.round(availableWidth.value * widthRatio.value), minPracticeWidth, maxWidth.value))
  const minHeight = computed(() => Math.min(180, Math.max(96, availableHeight.value - 120)))
  const maxHeight = computed(() => Math.max(minHeight.value, availableHeight.value - 180))
  const height = computed(() => clamp(savedHeight.value, minHeight.value, maxHeight.value))
  let observer: ResizeObserver | undefined
  let mounted = false
  function measure() {
    if (!workspace.value) return
    availableWidth.value = workspace.value.clientWidth
    availableHeight.value = workspace.value.clientHeight
  }
  function observe() {
    observer?.disconnect()
    if (!mounted || !workspace.value) return
    measure()
    observer = new ResizeObserver(measure)
    observer.observe(workspace.value)
  }
  function persist() {
    try { localStorage.setItem(storageKey, JSON.stringify({ placement: preferred.value, widthRatio: widthRatio.value, height: savedHeight.value })) } catch {}
  }
  function setPlacement(value: PracticePlacement) {
    if (value === 'right' && !sideAvailable.value) return
    preferred.value = value
    persist()
  }
  function setWidth(value: number) {
    if (Number.isFinite(value) && availableWidth.value > 0) widthRatio.value = clamp(value, minPracticeWidth, maxWidth.value) / availableWidth.value
  }
  function setHeight(value: number) {
    if (Number.isFinite(value)) savedHeight.value = clamp(value, minHeight.value, maxHeight.value)
  }
  watch(workspace, observe, { flush: 'post' })
  onMounted(() => {
    try {
      const value = JSON.parse(localStorage.getItem(storageKey) || 'null')
      if (value?.placement === 'right' || value?.placement === 'bottom') preferred.value = value.placement
      if (typeof value?.widthRatio === 'number' && Number.isFinite(value.widthRatio)) widthRatio.value = clamp(value.widthRatio, .05, .95)
      if (typeof value?.height === 'number' && Number.isFinite(value.height)) savedHeight.value = clamp(value.height, 96, 2000)
    } catch {}
    mounted = true
    observe()
  })
  onBeforeUnmount(() => { mounted = false; observer?.disconnect() })
  return { placement, sideAvailable, width, minWidth: minPracticeWidth, maxWidth, height, minHeight, maxHeight, setPlacement, setWidth, setHeight, persist }
}
