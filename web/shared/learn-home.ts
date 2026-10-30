import { contentUrl } from './content-url.ts'
export interface HomeSlide {
  id: string
  enabled: boolean
  label: string
  title: string
  description: string
  imageUrl: string
  cta: string
  href: string
}
export interface LearnHome {
  enabled: boolean
  autoplay: boolean
  intervalSeconds: number
  slides: HomeSlide[]
  lockVersion: number
}
export const defaultLearnHome: LearnHome = {
  enabled: true,
  autoplay: true,
  intervalSeconds: 6,
  slides: [
    {
      id: 'welcome',
      enabled: true,
      label: '开始探索',
      title: '从理解原理，到独立解决问题。',
      description: '选择一个模块，开始学习。',
      imageUrl: '',
      cta: '浏览学习模块',
      href: '/learn#module-list'
    }
  ],
  lockVersion: 0
}

export function parseLearnHome(raw: unknown): LearnHome {
  const v = raw as LearnHome
  if (
    !v ||
    typeof v.enabled !== 'boolean' ||
    typeof v.autoplay !== 'boolean' ||
    !Number.isInteger(v.intervalSeconds) ||
    v.intervalSeconds < 5 ||
    v.intervalSeconds > 30 ||
    !Number.isSafeInteger(v.lockVersion) ||
    v.lockVersion < 0 ||
    v.lockVersion > 2147483646 ||
    !Array.isArray(v.slides) ||
    v.slides.length > 10
  )
    throw Error('轮播配置无效；间隔应为 5–30 秒，最多 10 项')

  const ids = new Set<string>()
  const slides = v.slides.map(s => {
    if (!s || typeof s !== 'object' || typeof s.enabled !== 'boolean') throw Error('推荐项格式无效')
    const text = (key: keyof HomeSlide, max: number) => {
      const value = s[key] ?? ''
      if (typeof value !== 'string' || value.length > max) throw Error('推荐项字段格式无效或过长')
      return value.trim()
    }
    const out: HomeSlide = {
      id: text('id', 64),
      enabled: s.enabled,
      label: text('label', 30),
      title: text('title', 100),
      description: text('description', 300),
      imageUrl: contentUrl(text('imageUrl', 1024), '图片地址'),
      cta: text('cta', 30),
      href: contentUrl(text('href', 1024), '跳转链接')
    }
    if (!/^[a-zA-Z0-9_-]+$/.test(out.id) || ids.has(out.id) || !out.title)
      throw Error('标题必填，推荐项标识须有效且不可重复')
    ids.add(out.id)
    return out
  })
  return {
    enabled: v.enabled,
    autoplay: v.autoplay,
    intervalSeconds: v.intervalSeconds,
    slides,
    lockVersion: v.lockVersion
  }
}
