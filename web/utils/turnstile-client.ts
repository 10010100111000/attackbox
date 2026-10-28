export interface TurnstileApi {
  render(element: HTMLElement, options: Record<string, unknown>): string
  reset(id: string): void
  remove(id: string): void
}

export function turnstileApi() {
  return (window as typeof window & { turnstile?: TurnstileApi }).turnstile
}

let loading: Promise<void> | undefined
export function loadTurnstile() {
  if (turnstileApi()) return Promise.resolve()
  if (loading) return loading
  loading = new Promise<void>((resolve, reject) => {
    const script = document.createElement('script')
    let settled = false
    script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
    script.async = true
    script.defer = true
    const fail = () => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      loading = undefined
      script.remove()
      reject(Error('SDK unavailable'))
    }
    const timer = setTimeout(fail, 10000)
    script.onload = () => {
      if (settled) return
      if (!turnstileApi()) { fail(); return }
      settled = true
      clearTimeout(timer)
      resolve()
    }
    script.onerror = fail
    document.head.appendChild(script)
  })
  return loading
}
