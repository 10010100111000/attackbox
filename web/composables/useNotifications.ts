export function useNotifications() {
  const { currentUser } = useAuth()
  const state = useState('account:unread', () => ({ owner: '', count: 0 }))
  const unread = computed(() =>
    state.value.owner === currentUser.value?.id ? state.value.count : 0,
  )
  async function refreshUnread() {
    const owner = currentUser.value?.id
    if (!owner) {
      state.value = { owner: '', count: 0 }
      return
    }
    try {
      const r = await $fetch<{ count: number }>('/api/me/notifications/unread')
      if (currentUser.value?.id === owner)
        state.value = { owner, count: r.count }
    } catch {}
  }
  function setUnread(count: number) {
    if (currentUser.value) state.value = { owner: currentUser.value.id, count }
  }
  return { unread, refreshUnread, setUnread }
}
