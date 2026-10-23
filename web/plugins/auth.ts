export default defineNuxtPlugin(async () => {
  const { currentUser, isInitialized, fetchMe } = useAuth()
  if (import.meta.server) {
    currentUser.value = useRequestEvent()?.context.user || null
    isInitialized.value = true
    return
  }
  await fetchMe()
})
