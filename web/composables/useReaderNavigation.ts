import { ref, watch, onMounted, onBeforeUnmount, nextTick, type Ref } from 'vue'
import type { MarkdownHeading } from '~/utils/markdown'

interface Position {
  top: number
  section: string
  offset: number
}
export function useReaderNavigation(
  container: Ref<HTMLElement | null>,
  article: Ref<HTMLElement | null>,
  identity: Ref<string>,
  content: Ref<string>,
  sections: Ref<MarkdownHeading[]>
) {
  const activeSection = ref('')
  const route = useRoute(),
    router = useRouter()
  let mounted = false,
    ready = false,
    key = '',
    restoredKey = '',
    restoredContent = ''
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
    let id = ''
    for (const section of sections.value) {
      const element = find(section.id)
      if (element && topOf(element) <= container.value.scrollTop + 8) id = section.id
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
    await nextTick()
    if (ownGeneration !== generation || !container.value || !article.value) return
    if (listening !== container.value) {
      listening?.removeEventListener('scroll', scroll)
      listening = container.value
      listening.addEventListener('scroll', scroll, { passive: true })
    }
    const markdown = article.value.querySelector(':scope > .markdown-body')
    anchors = new Map(
      Array.from(markdown?.querySelectorAll<HTMLElement>('h2[id],h3[id],h4[id]') || []).map(el => [
        el.id,
        el
      ])
    )
    const quiz = article.value.querySelector<HTMLElement>('#quiz')
    if (quiz) anchors.set(quiz.id, quiz)
    if (restoredKey === identity.value && restoredContent === content.value) return
    key = identity.value
    ready = false
    let position: Position | null = null
    try {
      position = JSON.parse(localStorage.getItem(storageKey()) || 'null')
    } catch {
      /* 定位失败时保持当前位置,不打断阅读 */
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
    restoredContent = content.value
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
  watch(
    () => route.hash,
    () => {
      if (!ready) return
      if (!route.hash) container.value?.scrollTo({ top: 0, behavior: 'instant' })
      else jump(hashId())
      updateActive()
    }
  )
  async function goToSection(id: string) {
    if (!find(id)) return
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
