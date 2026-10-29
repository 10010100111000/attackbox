export interface HomeSlide {
  id: string
  enabled: boolean
  label: string
  title: string
  description: string
  cta: string
  href: string
  command: string
  caption: string
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
  autoplay: false,
  intervalSeconds: 6,
  slides: [
    {
      id: 'welcome',
      enabled: true,
      label: '开始探索',
      title: '从理解原理，到独立解决问题。',
      description: '选择一个模块，开始学习。',
      cta: '浏览学习模块',
      href: '/learn#module-list',
      command: '$ explore',
      caption: 'AttackBox Learning'
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
    if (!s || typeof s.enabled !== 'boolean') throw Error('推荐项格式无效')
    const out = {} as HomeSlide
    for (const [key, max] of Object.entries({
      id: 64,
      label: 30,
      title: 100,
      description: 300,
      cta: 30,
      href: 300,
      command: 100,
      caption: 100
    })) {
      const value = s[key as keyof HomeSlide]
      if (typeof value !== 'string' || value.length > max) throw Error('推荐项字段缺失或过长')
      ;(out as any)[key] = value.trim()
    }
    if (
      !/^[a-zA-Z0-9_-]+$/.test(out.id) ||
      ids.has(out.id) ||
      !out.title ||
      !out.cta ||
      !/^\/(?!\/)/.test(out.href) ||
      // eslint-disable-next-line no-control-regex -- 显式拒绝控制字符,防止不可见字符注入
      /[\\\s\u0000-\u001f?]/.test(out.href)
    )
      throw Error('标题、按钮必填，链接须为本站路径（不含查询参数），推荐项不可重复')
    ids.add(out.id)
    out.enabled = s.enabled
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
