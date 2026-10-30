import type { ReferenceReading } from '~/shared/references'

/** Owns a contiguous reading window. Distant outline jumps load their target
 * directly; scrolling continues from there without fetching skipped chapters. */
export function useReferenceChunks(
  document: ReferenceReading,
  container: Readonly<Ref<HTMLElement | null>>,
  sentinel: Ref<HTMLElement | null>
) {
  const chunks = shallowRef([{ index: 0, html: document.html }])
  const busy = ref(false),
    failure = ref(''),
    outdated = ref(false)
  const cache = new Map<number, string>([[0, document.html]])
  const requests = new Map<number, Promise<string>>()
  const abort = new AbortController()
  let observer: IntersectionObserver | undefined,
    mounted = false,
    generation = 0
  let retryAction: (() => Promise<string | undefined>) | undefined
  const first = computed(() => chunks.value[0]!.index)
  const last = computed(() => chunks.value.at(-1)!.index)
  const complete = computed(() => last.value === document.meta.chunks.length - 1)
  function html(index: number) {
    if (cache.has(index)) return Promise.resolve(cache.get(index)!)
    let request = requests.get(index)
    if (!request) {
      request = $fetch<string>(document.meta.chunks[index]!.url, {
        responseType: 'text',
        retry: false,
        signal: abort.signal
      })
        .then(value => {
          cache.set(index, value)
          return value
        })
        .finally(() => requests.delete(index))
      requests.set(index, request)
    }
    return request
  }
  async function move(
    index: number,
    mode: 'jump' | 'next' | 'previous',
    heading?: string
  ): Promise<string | undefined> {
    const own = ++generation
    busy.value = true
    failure.value = ''
    outdated.value = false
    try {
      const value = await html(index)
      if (own !== generation || abort.signal.aborted) return
      const height = container.value?.scrollHeight || 0
      const top = container.value?.scrollTop || 0
      if (mode === 'jump') chunks.value = [{ index, html: value }]
      else if (mode === 'next' && index === last.value + 1)
        chunks.value = [...chunks.value, { index, html: value }]
      else if (mode === 'previous' && index === first.value - 1)
        chunks.value = [{ index, html: value }, ...chunks.value]
      await nextTick()
      if (mode === 'previous' && container.value)
        container.value.scrollTop = top + container.value.scrollHeight - height
      retryAction = undefined
      return heading
    } catch (error) {
      if (own !== generation || abort.signal.aborted) return
      outdated.value = (error as { statusCode?: number }).statusCode === 404
      failure.value = outdated.value
        ? '此内容已不可用，请重新打开资料。'
        : '这一段暂时无法加载，请重试。'
      retryAction = outdated.value ? undefined : () => move(index, mode, heading)
    } finally {
      if (own === generation) busy.value = false
    }
  }
  async function ensureSection(id: string) {
    const heading = document.meta.outline.find(h => h.id === id)
    if (!heading || chunks.value.some(c => c.index === heading.chunk)) return
    await move(heading.chunk, 'jump', id)
  }
  async function next() {
    if (!busy.value && !failure.value && !complete.value) await move(last.value + 1, 'next')
  }
  async function previous() {
    if (!busy.value && first.value > 0) await move(first.value - 1, 'previous')
  }
  function observe() {
    observer?.disconnect()
    if (!mounted || !container.value || !sentinel.value) return
    observer = new IntersectionObserver(
      entries => {
        if (entries.some(e => e.isIntersecting)) void next()
      },
      { root: container.value, rootMargin: '0px 0px 160px 0px' }
    )
    observer.observe(sentinel.value)
  }
  watch([container, sentinel, last, busy], observe, { flush: 'post' })
  onMounted(() => {
    mounted = true
    observe()
  })
  onBeforeUnmount(() => {
    mounted = false
    abort.abort()
    observer?.disconnect()
  })
  return {
    chunks,
    busy,
    failure,
    outdated,
    first,
    complete,
    ensureSection,
    next,
    previous,
    retry: async () => {
      return await retryAction?.()
    }
  }
}
