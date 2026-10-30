import { analyticsPath } from '~/shared/analytics'
import type { AdPlacement } from '~/shared/advertising'

export default defineNuxtPlugin(nuxtApp => {
  const router = useRouter(),
    { currentUser } = useAuth()
  const storageKey = 'attackbox-visit-attribution'
  let mounted = false,
    lastPath = '',
    initial = true
  let current: { path: string; id: string; ready: Promise<boolean> } | null = null
  function source(query: Record<string, unknown>) {
    const scalar = (key: string, max: number) =>
      typeof query[key] === 'string' ? query[key].slice(0, max) : ''
    let host = ''
    if (initial) {
      try {
        host = new URL(document.referrer).hostname
      } catch {
        /* Direct navigation has no referrer. */
      }
    }
    const inbound = {
      referrerHost: host,
      source: scalar('utm_source', 120),
      medium: scalar('utm_medium', 80),
      campaign: scalar('utm_campaign', 120)
    }
    let attribution = inbound
    try {
      const saved = JSON.parse(sessionStorage.getItem(storageKey) || 'null')
      if (
        !inbound.source &&
        (!host || host === location.hostname) &&
        saved &&
        Date.now() - saved.lastSeen < 30 * 60 * 1000
      )
        attribution = saved.value
      sessionStorage.setItem(
        storageKey,
        JSON.stringify({ lastSeen: Date.now(), value: attribution })
      )
    } catch {
      /* Statistics also work without browser storage. */
    }
    initial = false
    return attribution
  }
  function start() {
    if (!mounted) return
    const route = router.currentRoute.value
    if (route.path === lastPath) return
    lastPath = route.path
    current = null
    if (currentUser.value?.role === 'admin' || navigator.doNotTrack === '1') return
    let path
    try {
      path = analyticsPath(route.path)
    } catch {
      return
    }
    const id = crypto.randomUUID()
    const body = { id, path, ...source(route.query) }
    const ready = fetch('/api/analytics/visits', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(body),
      keepalive: true
    })
      .then(async response => response.ok && (await response.json()).accepted === true)
      .catch(() => false)
    current = { id, path, ready }
  }
  function send(url: string, body: object) {
    const json = JSON.stringify(body)
    try {
      if (navigator.sendBeacon?.(url, new Blob([json], { type: 'application/json' }))) return
    } catch {
      /* Fall back to a non-blocking request. */
    }
    void fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: json,
      credentials: 'same-origin',
      keepalive: true
    }).catch(() => {})
  }
  async function record(url: string, path: string, body: object) {
    const visit = current
    if (visit?.path !== path || !(await visit.ready)) return
    send(url, { ...body, visitId: visit.id })
  }
  nuxtApp.hook('app:mounted', () => {
    mounted = true
    start()
  })
  nuxtApp.hook('page:finish', start)
  return {
    provide: {
      siteAnalytics: {
        read: (path: string) => record('/api/analytics/read', path, {}),
        advertisement: (
          path: string,
          adId: string,
          placement: AdPlacement,
          kind: 'impression' | 'click'
        ) => record('/api/analytics/advertisements', path, { adId, placement, kind })
      }
    }
  }
})
