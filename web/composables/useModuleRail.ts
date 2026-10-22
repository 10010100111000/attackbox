import {
  ref,
  computed,
  onMounted,
  onBeforeUnmount,
  getCurrentInstance,
  nextTick,
  type Ref,
  type ComputedRef
} from 'vue'

export interface ModuleRailOptions {
  cardSelector?: string
  speed?: number
  reducedMotionSpeed?: number
  autoStart?: boolean
}

export interface ModuleRailReturn {
  isPaused: ComputedRef<boolean>
  pause: () => void
  resume: (delayMs?: number) => void
  measure: () => void
  init: () => void
  destroy: () => void
  handleCardPointerMove: (e: PointerEvent, target?: HTMLElement | null) => void
}

const performanceNow = (): number => {
  if (typeof performance !== 'undefined' && typeof performance.now === 'function') {
    return performance.now()
  }
  return Date.now()
}

/**
 * Updates CSS custom properties `--mx` and `--my` on the card element
 * to create mouse-following gradient shimmer effects.
 */
export function handleCardPointerMove(e: PointerEvent, target?: HTMLElement | null): void {
  if (typeof window !== 'undefined' && window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    return
  }

  const card = target || (e.currentTarget as HTMLElement) || (e.target as HTMLElement)?.closest?.('[data-module]')
  if (!card || typeof (card as HTMLElement).getBoundingClientRect !== 'function' || !(card as HTMLElement).style) {
    return
  }

  const rect = (card as HTMLElement).getBoundingClientRect()
  ;(card as HTMLElement).style.setProperty('--mx', `${e.clientX - rect.left}px`)
  ;(card as HTMLElement).style.setProperty('--my', `${e.clientY - rect.top}px`)
}

export function useModuleRail(
  railRef: Ref<HTMLElement | null>,
  options: ModuleRailOptions = {}
): ModuleRailReturn {
  const cardSelector = options.cardSelector ?? '[data-module]'
  const baseSpeed = options.speed ?? 0.032
  const reducedSpeed = options.reducedMotionSpeed ?? 0.012

  const railHovered = ref(false)
  const railFocused = ref(false)
  const railVisible = ref(true)
  const isLoopRunning = ref(false)
  const isProgrammaticallyPaused = ref(false)

  let shelfPosition = 0
  let loopWidth = 0
  let resumeAt = 0
  let frameId: number | null = null
  let lastNow = 0
  let isDisposed = false

  let resizeObserver: ResizeObserver | null = null
  let visibilityObserver: IntersectionObserver | null = null

  const isPaused = computed(() => {
    return !isLoopRunning.value || isProgrammaticallyPaused.value || railHovered.value || railFocused.value || !railVisible.value
  })

  const cloneCards = () => {
    const rail = railRef.value
    if (!rail || typeof rail.querySelectorAll !== 'function') return

    // Find original cards that are not cloned copies
    const originals = Array.from(rail.querySelectorAll(`${cardSelector}:not([data-loop-copy])`))
    if (originals.length === 0) return

    // Clean up any stale loop copies
    const staleClones = rail.querySelectorAll('[data-loop-copy]')
    staleClones.forEach(el => {
      if (typeof el.remove === 'function') {
        el.remove()
      } else if (el.parentElement && typeof el.parentElement.removeChild === 'function') {
        el.parentElement.removeChild(el)
      }
    })

    // Clone each original card and mark with a11y & non-interactive attributes
    for (const card of originals) {
      if (typeof card.cloneNode !== 'function') continue
      const clone = card.cloneNode(true) as HTMLElement
      if (clone.dataset) {
        clone.dataset.loopCopy = ''
      } else {
        clone.setAttribute('data-loop-copy', '')
      }
      clone.setAttribute('aria-hidden', 'true')
      clone.tabIndex = -1

      // Strip duplicated ID attributes on clone and children to prevent duplicate IDs in DOM
      clone.removeAttribute?.('id')
      if (typeof clone.querySelectorAll === 'function') {
        clone.querySelectorAll('[id]').forEach(el => el.removeAttribute('id'))
        clone.querySelectorAll('a, button, input, select, textarea, [tabindex]').forEach(el => {
          el.setAttribute('tabindex', '-1')
        })
      }
      rail.appendChild(clone)
    }
  }

  const measureShelf = () => {
    const rail = railRef.value
    if (!rail || typeof rail.querySelectorAll !== 'function') return

    const originals = rail.querySelectorAll(`${cardSelector}:not([data-loop-copy])`)
    const clones = rail.querySelectorAll('[data-loop-copy]')
    if (originals.length === 0 || clones.length === 0) return

    const firstOriginal = originals[0] as HTMLElement
    const firstClone = clones[0] as HTMLElement

    if (typeof firstClone.offsetLeft === 'number' && typeof firstOriginal.offsetLeft === 'number') {
      const calculatedWidth = firstClone.offsetLeft - firstOriginal.offsetLeft
      loopWidth = calculatedWidth > 0 ? calculatedWidth : 0
    }

    shelfPosition = loopWidth > 0 ? shelfPosition % loopWidth : 0
    if (typeof rail.scrollLeft === 'number') {
      rail.scrollLeft = shelfPosition
    }
  }

  const advanceShelf = (now: number, dt: number) => {
    const rail = railRef.value
    if (!rail || !loopWidth || !railVisible.value || isProgrammaticallyPaused.value || railHovered.value || railFocused.value || now < resumeAt) {
      return
    }

    const isReduced = typeof window !== 'undefined' && window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches
    const speed = isReduced ? reducedSpeed : baseSpeed

    shelfPosition = ((shelfPosition + dt * speed) % loopWidth + loopWidth) % loopWidth
    rail.scrollLeft = shelfPosition
  }

  const tick = (now: number) => {
    if (isDisposed || !isLoopRunning.value) return
    frameId = requestAnimationFrame(tick)

    if (typeof document !== 'undefined' && document.hidden) {
      lastNow = 0
      return
    }

    if (!lastNow) {
      lastNow = now
      return
    }

    const dt = Math.min(now - lastNow, 60)
    lastNow = now
    advanceShelf(now, dt)
  }

  const startLoop = () => {
    if (isLoopRunning.value || isDisposed) return
    isLoopRunning.value = true
    lastNow = 0

    if (typeof window !== 'undefined' && typeof requestAnimationFrame !== 'undefined') {
      frameId = requestAnimationFrame(tick)
    }
  }

  const stopLoop = () => {
    isLoopRunning.value = false
    if (frameId !== null && typeof cancelAnimationFrame !== 'undefined') {
      cancelAnimationFrame(frameId)
      frameId = null
    }
    lastNow = 0
  }

  // Interactive event listeners
  const onPointerEnter = (e: PointerEvent) => {
    if (e.pointerType === 'mouse') {
      railHovered.value = true
    }
  }

  const onPointerLeave = () => {
    railHovered.value = false
    resumeAt = performanceNow() + 500
  }

  const onFocusIn = () => {
    railFocused.value = true
  }

  const onFocusOut = (e: FocusEvent) => {
    const rail = railRef.value
    railFocused.value = rail ? rail.contains(e.relatedTarget as Node) : false
    resumeAt = performanceNow() + 800
  }

  const onManualScroll = () => {
    resumeAt = performanceNow() + 2800
  }

  const onScroll = () => {
    if (railHovered.value || railFocused.value || performanceNow() < resumeAt) {
      if (railRef.value && typeof railRef.value.scrollLeft === 'number') {
        shelfPosition = railRef.value.scrollLeft
      }
    }
  }

  const onPageHide = (e: PageTransitionEvent) => {
    stopLoop()
    if (!e.persisted) {
      destroy()
    }
  }

  const onPageShow = (e: PageTransitionEvent) => {
    if (e.persisted && !isDisposed) {
      if (railVisible.value && (options.autoStart ?? true)) {
        startLoop()
      }
    }
  }

  const attachEvents = (rail: HTMLElement) => {
    rail.addEventListener('pointerenter', onPointerEnter as EventListener)
    rail.addEventListener('pointerleave', onPointerLeave)
    rail.addEventListener('focusin', onFocusIn)
    rail.addEventListener('focusout', onFocusOut as EventListener)
    rail.addEventListener('wheel', onManualScroll, { passive: true })
    rail.addEventListener('pointerdown', onManualScroll, { passive: true })
    rail.addEventListener('touchmove', onManualScroll, { passive: true })
    rail.addEventListener('keydown', onManualScroll)
    rail.addEventListener('scroll', onScroll, { passive: true })

    if (typeof window !== 'undefined') {
      window.addEventListener('pagehide', onPageHide)
      window.addEventListener('pageshow', onPageShow)
    }
  }

  const detachEvents = (rail: HTMLElement) => {
    rail.removeEventListener('pointerenter', onPointerEnter as EventListener)
    rail.removeEventListener('pointerleave', onPointerLeave)
    rail.removeEventListener('focusin', onFocusIn)
    rail.removeEventListener('focusout', onFocusOut as EventListener)
    rail.removeEventListener('wheel', onManualScroll)
    rail.removeEventListener('pointerdown', onManualScroll)
    rail.removeEventListener('touchmove', onManualScroll)
    rail.removeEventListener('keydown', onManualScroll)
    rail.removeEventListener('scroll', onScroll)
  }

  const init = () => {
    if (typeof window === 'undefined' || typeof document === 'undefined') return
    const rail = railRef.value
    if (!rail) return

    isDisposed = false
    cloneCards()
    measureShelf()
    attachEvents(rail)

    // ResizeObserver for responsive layout updates
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => {
        measureShelf()
      })
      resizeObserver.observe(rail)
    }

    // IntersectionObserver to sleep rAF loop when off-screen
    if (typeof IntersectionObserver !== 'undefined') {
      visibilityObserver = new IntersectionObserver((entries) => {
        const isIntersecting = entries[0]?.isIntersecting ?? false
        railVisible.value = isIntersecting
        if (isIntersecting) {
          if (options.autoStart ?? true) {
            startLoop()
          }
        } else {
          stopLoop()
        }
      }, { rootMargin: '100px' })
      visibilityObserver.observe(rail)
    } else {
      railVisible.value = true
      if (options.autoStart ?? true) {
        startLoop()
      }
    }
  }

  const destroy = () => {
    isDisposed = true
    stopLoop()

    if (resizeObserver) {
      resizeObserver.disconnect()
      resizeObserver = null
    }

    if (visibilityObserver) {
      visibilityObserver.disconnect()
      visibilityObserver = null
    }

    // Always clean up global window listeners unconditionally
    if (typeof window !== 'undefined') {
      window.removeEventListener('pagehide', onPageHide)
      window.removeEventListener('pageshow', onPageShow)
    }

    const rail = railRef.value
    if (rail && typeof rail.removeEventListener === 'function') {
      detachEvents(rail)
    }
  }

  const pause = () => {
    isProgrammaticallyPaused.value = true
    stopLoop()
  }

  const resume = (delayMs: number = 0) => {
    isProgrammaticallyPaused.value = false
    railHovered.value = false
    railFocused.value = false
    resumeAt = performanceNow() + delayMs
    startLoop()
  }

  if (getCurrentInstance()) {
    onMounted(() => {
      nextTick(() => {
        init()
      })
    })

    onBeforeUnmount(() => {
      destroy()
    })
  }

  return {
    isPaused,
    pause,
    resume,
    measure: measureShelf,
    init,
    destroy,
    handleCardPointerMove
  }
}
