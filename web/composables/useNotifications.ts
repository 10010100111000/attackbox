import { apiFetch } from '~/utils/api'
export function useNotifications() {
  const { currentUser } = useAuth()
  const version = useState('account:unread-version', () => 0)
  const state = useState('account:unread', () => ({ owner: '', count: 0 }))
  const unread = computed(() =>
    state.value.owner === currentUser.value?.id ? state.value.count : 0
  )
  async function refreshUnread() {
    const requestVersion = ++version.value
    const owner = currentUser.value?.id
    if (!owner) {
      state.value = { owner: '', count: 0 }
      return
    }
    try {
      const r = await apiFetch<{ count: number }>('/api/me/notifications/unread')
      if (currentUser.value?.id === owner && version.value === requestVersion)
        state.value = { owner, count: r.count }
    } catch {
      /* 浏览器不支持通知或被用户拒绝时静默降级 */
    }
  }
  function setUnread(count: number) {
    version.value++
    if (currentUser.value) state.value = { owner: currentUser.value.id, count }
  }
  return { unread, refreshUnread, setUnread }
}
