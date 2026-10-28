import { defineEventHandler } from 'h3'
import { turnstileConfig } from '../../utils/turnstile'

export default defineEventHandler(() => {
  const { enabled, siteKey } = turnstileConfig()
  return { enabled, siteKey }
})
