// Loaded only in the isolated integration server; production has no provider URL override.
const url = new URL(process.env.TURNSTILE_CAPTURE_URL)
if (process.env.NODE_ENV !== 'test' || url.hostname !== '127.0.0.1')
  throw Error('local test capture required')
const original = globalThis.fetch
globalThis.fetch = (input, options) =>
  original(
    String(input) === 'https://challenges.cloudflare.com/turnstile/v0/siteverify' ? url : input,
    options
  )
