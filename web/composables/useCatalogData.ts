import type { MaybeRefOrGetter } from 'vue'
// A short, request-scoped Nuxt cache avoids refetching catalogues on every tab visit.
// Personalized callers must include the current identity in their key.
export function useCatalogData<T>(key: MaybeRefOrGetter<string>, url: string) {
  const fetchedAt = useState<Record<string, number>>('catalog:fetched-at', () => ({}))
  const request = useRequestFetch()
  return useLazyAsyncData<T>(key, async () => {
    const identity = toValue(key)
    const result = await request<T>(url, { retry: false })
    fetchedAt.value[identity] = Date.now()
    return result as T
  }, {
    getCachedData(cacheKey, app, context) {
      if (app.isHydrating) return app.payload.data[cacheKey]
      const age = Date.now() - (fetchedAt.value[cacheKey] || 0)
      if (context.cause !== 'initial' || age < 0 || age >= 45_000) return undefined
      return app.payload.data[cacheKey] ?? app.static.data[cacheKey]
    },
  })
}
