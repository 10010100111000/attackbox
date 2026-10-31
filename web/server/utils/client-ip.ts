import { BlockList, isIP } from 'node:net'
import { createError, getRequestIP, getHeader, type H3Event } from 'h3'

function normalize(address: string) {
  return /^::ffff:/i.test(address) && isIP(address.slice(7)) === 4 ? address.slice(7) : address
}

let cached: { raw: string; list: BlockList | null } | undefined

function proxyList(raw: string) {
  if (cached?.raw === raw) {
    if (!cached.list) throw invalidProxyConfig()
    return cached.list
  }
  const list = new BlockList()
  try {
    for (const item of raw.split(',').map(value => value.trim())) {
      const [network, bits, ...extra] = item.split('/')
      const family = isIP(network || '')
      if (
        extra.length ||
        !family ||
        bits === undefined ||
        !/^\d+$/.test(bits) ||
        Number(bits) > (family === 4 ? 32 : 128)
      ) {
        throw Error('invalid CIDR')
      }
      list.addSubnet(network!, Number(bits), family === 4 ? 'ipv4' : 'ipv6')
    }
  } catch {
    cached = { raw, list: null }
    throw invalidProxyConfig()
  }
  cached = { raw, list }
  return list
}

function invalidProxyConfig() {
  return createError({ statusCode: 503, message: '可信代理配置无效，请联系管理员' })
}

export function trustedProxy(
  address: string,
  raw = process.env.AUTH_TRUSTED_PROXY_CIDRS || '127.0.0.0/8,::1/128'
) {
  const list = proxyList(raw)
  const peer = normalize(address)
  return isIP(peer) !== 0 && list.check(peer, isIP(peer) === 4 ? 'ipv4' : 'ipv6')
}

export function clientIp(event: H3Event) {
  const peer =
    event.node.req.socket.remoteAddress || getRequestIP(event, { xForwardedFor: false }) || ''
  if (trustedProxy(peer)) {
    // nginx overwrites these headers; never trust CF-Connecting-IP in the app.
    const real = getHeader(event, 'x-real-ip')?.trim() || ''
    if (isIP(real)) return normalize(real)
    const chain = (getHeader(event, 'x-forwarded-for') || '')
      .split(',')
      .map(ip => normalize(ip.trim()))
    if (chain.every(ip => isIP(ip))) {
      // Stop at the first untrusted hop. Addresses to its left may be spoofed.
      let address = normalize(peer)
      for (let index = chain.length - 1; index >= 0 && trustedProxy(address); index--) {
        address = chain[index]!
      }
      return address
    }
  }
  return normalize(peer)
}
