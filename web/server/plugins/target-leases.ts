import { sandboxConfig } from '../sandbox/config'
import { reconcileTargetLeases } from '../services/target-leases'
export default defineNitroPlugin(nitro => {
  if (import.meta.prerender || !sandboxConfig().enabled) return
  let running = false,
    closed = false
  async function sweep() {
    if (running || closed) return
    running = true
    try {
      await reconcileTargetLeases()
    } catch {
      console.error('[targets] lease reconciliation failed; will retry')
    } finally {
      running = false
    }
  }
  const timer = setInterval(() => void sweep(), 15000)
  timer.unref()
  nitro.hooks.hook('close', () => {
    closed = true
    clearInterval(timer)
  })
})
