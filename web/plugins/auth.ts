export default defineNuxtPlugin(nuxtApp => {
  const { currentUser, isInitialized, fetchMe } = useAuth()
  if (import.meta.server) {
    currentUser.value = useRequestEvent()?.context.user || null
    isInitialized.value = true
    return
  }

  // Dynamic SSR already resolved this request's session. Prerendered HTML cannot
  // contain a visitor's identity; restore it after mounting without blocking forms.
  if (nuxtApp.payload.prerenderedAt || !isInitialized.value) {
    isInitialized.value = false
    nuxtApp.hook('app:mounted', () => { void fetchMe() })
  }
})
