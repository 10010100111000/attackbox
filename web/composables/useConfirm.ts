import { getCurrentScope, onScopeDispose } from 'vue'
import type { ConfirmationOptions } from '~/utils/confirmation'

export function useConfirm() {
  const { $confirmation } = useNuxtApp()
  const { currentUser } = useAuth()
  const route = useRoute()
  const owner = Symbol('confirmation-owner')
  let disposed = false
  if (getCurrentScope()) onScopeDispose(() => { disposed = true; $confirmation.controller.cancel(owner) })
  return (options: ConfirmationOptions): Promise<boolean> => {
    if (import.meta.server || disposed) return Promise.resolve(false)
    const actor = currentUser.value?.id
    const path = route.fullPath
    const trigger = document.activeElement
    return $confirmation.controller.ask(options, owner, () => {
      if (trigger instanceof HTMLElement && trigger.isConnected) trigger.focus({ preventScroll: true })
    }).then(accepted => accepted && !disposed && currentUser.value?.id === actor && route.fullPath === path)
  }
}
