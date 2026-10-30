// Chapter and reference readers own nested scroll containers. Delegate other routes
// to Nuxt's normal scroll behavior, including its page-loading synchronization.
export default defineNuxtPlugin(() => {
  const router = useRouter()
  const original = router.options.scrollBehavior
  router.options.scrollBehavior = (to, from, saved) =>
    /^\/learn\/modules\/[^/]+\/[^/]+\/?$/.test(to.path) || /^\/references\/[^/]+\/?$/.test(to.path)
      ? false
      : original?.(to, from, saved)
})
