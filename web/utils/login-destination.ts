/** Only internal page destinations; malformed or looping redirects use Learn. */
export function loginDestination(value: unknown): string {
  if (
    typeof value !== 'string' ||
    !value.startsWith('/') ||
    value.startsWith('//') ||
    // eslint-disable-next-line no-control-regex -- 显式拒绝控制字符,防止不可见字符注入
    /[\\\u0000-\u0020]/.test(value)
  )
    return '/learn'
  try {
    const base = 'https://attackbox.invalid'
    const url = new URL(value, base)
    const path = decodeURIComponent(url.pathname)
    if (
      url.origin !== base ||
      path.startsWith('//') ||
      // eslint-disable-next-line no-control-regex -- 显式拒绝控制字符,防止不可见字符注入
      /[\\\u0000-\u0020]/.test(path) ||
      path === '/auth' ||
      path.startsWith('/auth/')
    )
      return '/learn'
    return url.pathname + url.search + url.hash
  } catch {
    return '/learn'
  }
}
