import { BlockList, isIP } from 'node:net'
import { createError, getRequestIP, getHeader, type H3Event } from 'h3'

function normalize(address: string) {
  return address.startsWith('::ffff:') && isIP(address.slice(7)) === 4 ? address.slice(7) : address
}

export function trustedProxy(address: string, raw = process.env.AUTH_TRUSTED_PROXY_CIDRS || '127.0.0.0/8,::1/128') {
  const list = new BlockList()
  try {
    for (const item of raw.split(',').map(value => value.trim())) {
      const [network, bits, ...extra] = item.split('/')
      const family = isIP(network || '')
      if (extra.length || !family || bits === undefined || !/^\d+$/.test(bits) || Number(bits) > (family === 4 ? 32 : 128)) {
        throw Error('invalid CIDR')
      }
      list.addSubnet(network!, Number(bits), family === 4 ? 'ipv4' : 'ipv6')
    }
  } catch {
    throw createError({ statusCode: 503, message: '可信代理配置无效，请联系管理员' })
  }
  const peer = normalize(address)
  return isIP(peer) !== 0 && list.check(peer, isIP(peer) === 4 ? 'ipv4' : 'ipv6')
}

export function clientIp(event: H3Event) {
  const peer = event.node.req.socket.remoteAddress || getRequestIP(event, { xForwardedFor: false }) || ''
  if (trustedProxy(peer)) {
    // nginx overwrites these headers; never trust CF-Connecting-IP in the app.
    const forwarded = getHeader(event, 'x-real-ip') || getHeader(event, 'x-forwarded-for') || ''
    if (isIP(forwarded)) return normalize(forwarded)
  }
  return normalize(peer)
}
