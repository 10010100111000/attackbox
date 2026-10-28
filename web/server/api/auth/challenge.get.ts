import { defineEventHandler, setResponseHeader } from 'h3'
import { turnstileConfig } from '../../utils/turnstile'

export default defineEventHandler(event => {
  setResponseHeader(event, 'Cache-Control', 'no-store')
  const { enabled, siteKey } = turnstileConfig()
  return { enabled, siteKey }
})
