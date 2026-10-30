import { contentUrl } from './content-url.ts'
export const adPlacements = [
  { value: 'learn_catalog', label: '学习目录页' },
  { value: 'reference_footer', label: '参考资料文末' }
] as const
export type AdPlacement = (typeof adPlacements)[number]['value']
export interface AdInput {
  name: string
  title: string
  description: string
  imageUrl: string
  href: string
  cta: string
  sponsor: string
  placements: AdPlacement[]
  enabled: boolean
  weight: number
  startsAt: string | null
  endsAt: string | null
  lockVersion: number
}
export interface Advertisement extends AdInput {
  id: string
}
export interface AdCreative {
  id: string
  title: string
  description: string
  imageUrl: string
  href: string
  cta: string
  sponsor: string
}
export function emptyAd(): AdInput {
  return {
    name: '',
    title: '',
    description: '',
    imageUrl: '',
    href: '',
    cta: '',
    sponsor: '',
    placements: ['learn_catalog'],
    enabled: false,
    weight: 1,
    startsAt: null,
    endsAt: null,
    lockVersion: 0
  }
}
export function isAdPlacement(value: unknown): value is AdPlacement {
  return adPlacements.some(placement => placement.value === value)
}
export function parseAd(raw: unknown): AdInput {
  if (!raw || typeof raw !== 'object') throw Error('广告配置无效')
  const v = raw as Record<string, unknown>
  const text = (key: string, max: number) => {
    const value = v[key] ?? ''
    if (typeof value !== 'string' || value.length > max) throw Error('广告字段格式无效或过长')
    return value.trim()
  }
  const time = (key: string) => {
    const value = v[key]
    if (value === null || value === undefined || value === '') return null
    if (typeof value !== 'string' || !Number.isFinite(Date.parse(value)))
      throw Error('展示时间无效')
    return new Date(value).toISOString()
  }
  const ad: AdInput = {
    name: text('name', 120),
    title: text('title', 120),
    description: text('description', 300),
    imageUrl: contentUrl(text('imageUrl', 1024), '图片地址'),
    href: contentUrl(text('href', 1024), '跳转链接'),
    cta: text('cta', 30),
    sponsor: text('sponsor', 80),
    placements: Array.isArray(v.placements) ? v.placements.filter(isAdPlacement) : [],
    enabled: v.enabled === true,
    weight: Number(v.weight),
    startsAt: time('startsAt'),
    endsAt: time('endsAt'),
    lockVersion: Number(v.lockVersion)
  }
  if (!ad.name || !ad.title || !ad.href) throw Error('内部名称、标题与跳转链接必填')
  if (
    typeof v.enabled !== 'boolean' ||
    !Number.isInteger(ad.weight) ||
    ad.weight < 1 ||
    ad.weight > 100 ||
    !Number.isSafeInteger(ad.lockVersion) ||
    ad.lockVersion < 0 ||
    ad.lockVersion > 2147483646
  )
    throw Error('广告状态、权重或版本无效')
  if (
    !ad.placements.length ||
    !Array.isArray(v.placements) ||
    ad.placements.length !== v.placements.length ||
    new Set(ad.placements).size !== ad.placements.length
  )
    throw Error('请选择有效且不重复的广告位')
  if (ad.startsAt && ad.endsAt && ad.startsAt >= ad.endsAt) throw Error('结束时间须晚于开始时间')
  return ad
}
export function adState(ad: Pick<AdInput, 'enabled' | 'startsAt' | 'endsAt'>, now = Date.now()) {
  if (!ad.enabled) return '已停用'
  if (ad.startsAt && Date.parse(ad.startsAt) > now) return '待开始'
  if (ad.endsAt && Date.parse(ad.endsAt) <= now) return '已结束'
  return '展示中'
}
