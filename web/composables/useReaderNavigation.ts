import { ref, watch, onMounted, onBeforeUnmount, nextTick, type Ref } from 'vue'
import type { MarkdownHeading } from '~/utils/markdown'

interface Position {
  top: number
  section: string
  offset: number
}
export function useReaderNavigation(
  container: Readonly<Ref<HTMLElement | null>>,
  article: Readonly<Ref<HTMLElement | null>>,
  identity: Ref<string>,
  content: Ref<string>,
  sections: Ref<MarkdownHeading[]>,
  prepareSection?: (id: string) => Promise<void>
) {
  const activeSection = ref('')
  const route = useRoute(),
    router = useRouter()
  let mounted = false,
    ready = false,
    key = '',
    restoredKey = ''
  let frame = 0,
    timer: ReturnType<typeof setTimeout> | undefined
  let listening: HTMLElement | null = null,
    generation = 0
  let anchors = new Map<string, HTMLElement>()
  const storageKey = () => `attackbox-reader-position:${key}`
  function find(id: string) {
    return anchors.get(id)
  }
  function topOf(el: HTMLElement) {
    return (
      container.value!.scrollTop +
      el.getBoundingClientRect().top -
      container.value!.getBoundingClientRect().top -
      24
    )
  }
  function updateActive() {
    frame = 0
    if (!container.value || !ready) return
    const { scrollTop, scrollHeight, clientHeight } = container.value
    const atEnd = scrollTop > 0 && scrollHeight - clientHeight - scrollTop <= 2
    let id = ''
    for (const section of sections.value) {
      const element = find(section.id)
      // A short final section cannot reach the top: scrolling is clamped at the
      // document end. Include visible trailing headings in that boundary case.
      if (
        element &&
        (topOf(element) <= scrollTop + 8 || (atEnd && topOf(element) < scrollTop + clientHeight))
      )
        id = section.id
    }
    activeSection.value = id
  }
  function save() {
    clearTimeout(timer)
    if (!ready || !key || !container.value) return
    const element = find(activeSection.value)
    const value: Position = {
      top: container.value.scrollTop,
      section: activeSection.value,
      offset: element ? container.value.scrollTop - topOf(element) : 0
    }
    try {
      localStorage.setItem(storageKey(), JSON.stringify(value))
    } catch {
      /* 定位失败时保持当前位置,不打断阅读 */
    }
  }
  function scroll() {
    if (!ready) return
    if (!frame) frame = requestAnimationFrame(updateActive)
    clearTimeout(timer)
    timer = setTimeout(save, 350)
  }
  function jump(id: string) {
    const target = find(id)
    if (!target || !container.value) return false
    container.value.scrollTo({ top: Math.max(0, topOf(target)), behavior: 'instant' })
    updateActive()
    return true
  }
  function hashId() {
    try {
      return decodeURIComponent(route.hash.slice(1))
    } catch {
      return ''
    }
  }
  async function restore() {
    if (!mounted || !container.value || !article.value || !identity.value) return
    const ownGeneration = ++generation
    const restoring = restoredKey !== identity.value
    let position: Position | null = null
    if (restoring) {
      key = identity.value
      try {
        position = JSON.parse(localStorage.getItem(storageKey()) || 'null')
      } catch {
        /* Optional reading history. */
      }
      const target = hashId() || position?.section
      if (target) await prepareSection?.(target)
    }
    await nextTick()
    if (ownGeneration !== generation || !container.value || !article.value) return
    if (listening !== container.value) {
      listening?.removeEventListener('scroll', scroll)
      listening = container.value
      listening.addEventListener('scroll', scroll, { passive: true })
    }
    const markdown = article.value.querySelector(':scope > .markdown-body')
    anchors = new Map(
      Array.from(
        markdown?.querySelectorAll<HTMLElement>('h1[id],h2[id],h3[id],h4[id],h5[id],h6[id]') || []
      ).map(el => [el.id, el])
    )
    const quiz = article.value.querySelector<HTMLElement>('#quiz')
    if (quiz) anchors.set(quiz.id, quiz)
    if (!restoring) {
      updateActive()
      return
    }
    ready = true
    if (!hashId() || !jump(hashId())) {
      const element = typeof position?.section === 'string' ? find(position.section) : undefined
      const top =
        element && Number.isFinite(position?.offset)
          ? topOf(element) + position!.offset
          : position?.top
      container.value.scrollTo({
        top: typeof top === 'number' && Number.isFinite(top) ? Math.max(0, top) : 0,
        behavior: 'instant'
      })
    }
    restoredKey = identity.value
    updateActive()
  }
  watch(
    identity,
    () => {
      save()
      ready = false
      restoredKey = ''
      generation++
    },
    { flush: 'pre' }
  )
  watch([container, article, identity, content, sections], restore, { flush: 'post' })
  async function reveal(id: string) {
    await prepareSection?.(id)
    await restore()
    return jump(id)
  }
  watch(
    () => route.hash,
    async () => {
      if (!mounted) return
      if (!route.hash) container.value?.scrollTo({ top: 0, behavior: 'instant' })
      else await reveal(hashId())
      updateActive()
    }
  )
  async function goToSection(id: string) {
    if (!(await reveal(id))) return
    await router.replace({ path: route.path, query: route.query, hash: `#${id}` })
    jump(id)
    save()
  }
  function hide() {
    if (document.hidden) save()
  }
  onMounted(() => {
    mounted = true
    void restore()
    window.addEventListener('pagehide', save)
    document.addEventListener('visibilitychange', hide)
  })
  onBeforeUnmount(() => {
    save()
    mounted = false
    generation++
    cancelAnimationFrame(frame)
    clearTimeout(timer)
    listening?.removeEventListener('scroll', scroll)
    window.removeEventListener('pagehide', save)
    document.removeEventListener('visibilitychange', hide)
  })
  return { activeSection, goToSection }
}
