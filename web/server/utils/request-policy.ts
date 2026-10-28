import { createError, getHeader, getRequestURL, getResponseHeader, getResponseStatus, setResponseHeader, type H3Event } from 'h3'

const safeMethods = new Set(['GET', 'HEAD', 'OPTIONS'])
const renderedResponses = new WeakSet<H3Event>()
export const privateCacheControl = 'private, no-store'

// H3 has already decoded event.path for routing. Use the same path and match a
// namespace boundary, rather than feature prefixes or the raw encoded URL.
export function isApiPath(path: string) {
  const pathname = path.split('?', 1)[0] || '/'
  return pathname === '/api' || pathname.startsWith('/api/')
}

export function cookieVary(...values: unknown[]) {
  const tokens = new Map<string, string>()
  for (const value of values.flat()) {
    if (value === undefined || value === null) continue
    for (const token of String(value).split(',').map(token => token.trim()).filter(Boolean)) {
      if (token === '*') return '*'
      if (!tokens.has(token.toLowerCase())) tokens.set(token.toLowerCase(), token)
    }
  }
  if (!tokens.has('cookie')) tokens.set('cookie', 'Cookie')
  return [...tokens.values()].join(', ')
}

function privateHeaders(event: H3Event) {
  setResponseHeader(event, 'Cache-Control', privateCacheControl)
  setResponseHeader(event, 'Vary', cookieVary(getResponseHeader(event, 'Vary')))
}

export function guardApiRequest(event: H3Event) {
  if (!isApiPath(event.path)) return
  // Apply before authentication/handlers so rejected operations are private too.
  privateHeaders(event)
  if (safeMethods.has(event.method)) return
  const origin = getHeader(event, 'origin')
  if (getHeader(event, 'sec-fetch-site')?.toLowerCase() === 'cross-site'
    || (origin && origin !== getRequestURL(event).origin)) {
    throw createError({ statusCode: 403, message: '请从本站提交操作' })
  }
  // Server-to-server calls without browser origin metadata retain their route's
  // existing authentication requirements; this guard does not grant access.
}

export function markRenderedResponse(event: H3Event) {
  // Includes Nuxt's JSON payload responses, not only text/html documents.
  renderedResponses.add(event)
}

export function applyResponseCachePolicy(event: H3Event, response?: { body?: unknown }) {
  const body = response?.body
  const webResponse = body instanceof Response ? body : undefined
  const contentType = String(webResponse?.headers.get('content-type') || getResponseHeader(event, 'Content-Type') || '')
  const status = webResponse?.status || getResponseStatus(event)
  const html = contentType.split(';', 1)[0]?.trim().toLowerCase() === 'text/html'
    || (!contentType && typeof body === 'string' && status !== 204 && status !== 304)
  if (!isApiPath(event.path) && !renderedResponses.has(event) && !html) return
  privateHeaders(event)
  if (webResponse && response) {
    // Response headers are sent after Nitro's beforeResponse hook. Preserve the
    // policy for these handlers too, without consuming or buffering the body.
    const headers = new Headers(webResponse.headers)
    headers.set('Cache-Control', privateCacheControl)
    headers.set('Vary', cookieVary(getResponseHeader(event, 'Vary'), headers.get('Vary')))
    response.body = new Response(webResponse.body, { status: webResponse.status, statusText: webResponse.statusText, headers })
  }
}
