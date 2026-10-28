import { createError } from 'h3'
import { feedbackCategories, feedbackStatuses, type FeedbackCategory, type FeedbackStatus } from '../../shared/feedback'

export function feedbackInput(input: unknown) {
  const value = input as Record<string, unknown> | null
  const category = value?.category
  const body = typeof value?.body === 'string' ? value.body.trim() : ''
  if (!feedbackCategories.some(option => option.value === category) || !body || body.length > 4000 || body.includes('\0')) {
    throw createError({ statusCode: 400, message: '请选择反馈类型，内容需为 1–4000 字' })
  }
  let sourcePath: string | null = null
  if (value?.sourcePath !== undefined && value.sourcePath !== null && value.sourcePath !== '') {
    if (typeof value.sourcePath !== 'string') throw createError({ statusCode: 400, message: '反馈页面路径无效' })
    // Keep only the pathname, never query strings that could contain tokens or personal details.
    sourcePath = value.sourcePath.split(/[?#]/, 1)[0] || ''
    let decoded: string
    try { decoded = decodeURIComponent(sourcePath) } catch { decoded = '' }
    if (!sourcePath.startsWith('/') || sourcePath.startsWith('//') || sourcePath.length > 500
      || !decoded || decoded.startsWith('//') || /[\\\s\u0000-\u001f\u007f]/.test(decoded)) {
      throw createError({ statusCode: 400, message: '反馈页面仅支持本站路径' })
    }
  }
  return { category: category as FeedbackCategory, body, sourcePath }
}

export function feedbackStatus(value: unknown): FeedbackStatus {
  if (!feedbackStatuses.some(option => option.value === value)) {
    throw createError({ statusCode: 400, message: '反馈状态无效' })
  }
  return value as FeedbackStatus
}

export function feedbackId(value: unknown) {
  if (typeof value !== 'string' || !/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/i.test(value)) {
    throw createError({ statusCode: 400, message: '反馈标识无效' })
  }
  return value
}
