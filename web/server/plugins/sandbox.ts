import { sandboxHub, sandboxController } from '../sandbox'
export default defineNitroPlugin(nitro => {
  const hub = sandboxHub()
  sandboxController()
  const timer = setInterval(() => hub.sweep(), 3000)
  timer.unref()
  nitro.hooks.hook('close', () => { clearInterval(timer); hub.shutdown() })
})
