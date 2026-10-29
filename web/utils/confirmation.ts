export interface ConfirmationOptions {
  title: string
  description: string
  confirmLabel?: string
  cancelLabel?: string
  tone?: 'danger' | 'neutral'
}
export interface ConfirmationRequest extends ConfirmationOptions {
  id: number
}

// One app, one decision. Never queue actions that might be stale by the time
// an earlier dialog closes. The caller's scope owns the pending request.
export function createConfirmationController(
  publish: (request: ConfirmationRequest | null) => void
) {
  let sequence = 0
  let pending: {
    id: number
    owner: symbol
    resolve: (value: boolean) => void
    restore?: () => void
  } | null = null
  function finish(id: number, accepted: boolean) {
    if (!pending || pending.id !== id) return
    const request = pending
    pending = null
    publish(null)
    // Let Vue remove the focus trap before restoring the initiating control.
    queueMicrotask(() => {
      try {
        request.restore?.()
      } finally {
        request.resolve(accepted)
      }
    })
  }
  return {
    ask(options: ConfirmationOptions, owner: symbol, restore?: () => void): Promise<boolean> {
      if (pending) return Promise.resolve(false)
      const id = ++sequence
      return new Promise(resolve => {
        pending = { id, owner, resolve, restore }
        publish({ ...options, id })
      })
    },
    finish,
    cancel(owner?: symbol) {
      if (pending && (!owner || owner === pending.owner)) finish(pending.id, false)
    }
  }
}
