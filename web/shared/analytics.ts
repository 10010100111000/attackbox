export interface VisitInput {
  id: string
  path: string
  referrerHost: string
  source: string
  medium: string
  campaign: string
}
export interface TrafficRow {
  path: string
  views: number
  visitors: number
  reads: number
}
export interface SourceRow {
  source: string
  medium: string
  campaign: string
  channel: string
  views: number
  visitors: number
  reads: number
}
export interface AnalyticsReport {
  days: number
  totals: {
    views: number
    visitors: number
    reads: number
    impressions: number
    clicks: number
    newAccounts: number
    completedChapters: number
  }
  daily: { day: string; views: number; visitors: number; reads: number }[]
  sources: SourceRow[]
  pages: TrafficRow[]
  advertisements: {
    id: string
    name: string
    placement: string
    impressions: number
    clicks: number
  }[]
}
export function analyticsPath(value: unknown): string {
  if (
    typeof value !== 'string' ||
    value.length > 500 ||
    /[?#\\\s]/.test(value) ||
    !/^\/(?:learn(?:\/(?:modules\/[^/]+(?:\/[^/]+)?|routes\/[^/]+))?|references(?:\/[^/]+)?|challenges(?:\/[^/]+)?|auth)?\/?$/.test(
      value
    )
  )
    throw Error('访问路径无效')
  const path = new URL(value, 'https://analytics.invalid').pathname
  if (path !== value) throw Error('访问路径无效')
  return path
}
export function isAnalyticsId(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i.test(value)
  )
}
export function analyticsId(value: unknown): string {
  if (!isAnalyticsId(value)) throw Error('统计标识无效')
  return value
}
export function parseVisit(raw: unknown): VisitInput {
  if (!raw || typeof raw !== 'object') throw Error('访问记录无效')
  const v = raw as Record<string, unknown>
  const text = (key: string, max: number) => {
    const value = v[key] ?? ''
    // eslint-disable-next-line no-control-regex
    if (typeof value !== 'string' || value.length > max || /[\u0000-\u001f\u007f]/.test(value))
      throw Error('访问来源格式无效')
    return value.trim()
  }
  const host = text('referrerHost', 253).toLowerCase()
  if (host && !/^[a-z0-9.-]+$/.test(host)) throw Error('来源域名无效')
  return {
    id: analyticsId(v.id),
    path: analyticsPath(v.path),
    referrerHost: host,
    source: text('source', 120),
    medium: text('medium', 80),
    campaign: text('campaign', 120)
  }
}
export function visitSource(
  input: Pick<VisitInput, 'referrerHost' | 'source' | 'medium' | 'campaign'>,
  siteHost: string
) {
  if (input.source)
    return {
      source: input.source,
      medium: input.medium || 'campaign',
      campaign: input.campaign,
      channel: 'campaign'
    }
  const host = input.referrerHost.replace(/^www\./, '')
  if (!host || host === siteHost.toLowerCase().replace(/^www\./, ''))
    return { source: '(direct)', medium: '', campaign: '', channel: 'direct' }
  const searches = [
    'google.com',
    'google.de',
    'google.co.uk',
    'google.com.hk',
    'google.com.tw',
    'google.co.jp',
    'bing.com',
    'baidu.com',
    'duckduckgo.com',
    'yandex.com'
  ]
  return {
    source: host,
    medium: '',
    campaign: '',
    channel: searches.some(domain => host === domain || host.endsWith('.' + domain))
      ? 'search'
      : 'referral'
  }
}
