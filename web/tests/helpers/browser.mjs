import { existsSync } from 'node:fs'

export function chromiumOptions() {
  return {
    executablePath:
      process.env.CHROMIUM_PATH ||
      (existsSync('/usr/bin/chromium') ? '/usr/bin/chromium' : undefined),
    args: ['--no-sandbox']
  }
}
