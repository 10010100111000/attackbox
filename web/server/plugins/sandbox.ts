import { sandboxHub, sandboxController } from '../sandbox'
import { loadSandboxSettings } from '../services/sandbox-settings'
export default defineNitroPlugin(async nitro => {
  const hub = sandboxHub()
  await loadSandboxSettings()
  sandboxController()
  const timer = setInterval(() => hub.sweep(), 3000)
  timer.unref()
  nitro.hooks.hook('close', () => { clearInterval(timer); hub.shutdown() })
})
