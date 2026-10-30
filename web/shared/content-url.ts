/** Content links are browser-facing addresses, never server fetch instructions. */
export function contentUrl(value: string, field: string) {
  if (!value) return ''
  // eslint-disable-next-line no-control-regex
  if (/[\\\s\u0000-\u001f\u007f]/.test(value)) throw Error(`${field}不能含空白、控制字符或反斜线`)
  if (/^\/(?!\/)/.test(value)) return value
  try {
    const url = new URL(value)
    if (
      /^https:\/\//i.test(value) &&
      url.protocol === 'https:' &&
      url.hostname &&
      !url.username &&
      !url.password
    )
      return value
  } catch {
    /* Malformed URLs share the validation error below. */
  }
  throw Error(`${field}须为本站路径或完整 HTTPS 地址`)
}
