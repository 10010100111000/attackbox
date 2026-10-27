// Test process only. No configurable provider URL exists in the application.
const url = new URL(process.env.RESEND_CAPTURE_URL)
if (process.env.NODE_ENV !== 'test' || url.hostname !== '127.0.0.1') throw Error('local test capture required')
const original = globalThis.fetch
globalThis.fetch = (input, options) => original(String(input) === 'https://api.resend.com/emails' ? url : input, options)
