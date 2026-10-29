/**
 * 统一的 API 请求层。
 *
 * 约定:
 * - 请求默认 `retry: false`。写操作失败由界面明确提示,不能被自动重放。
 * - 失败统一抛 ApiError;界面只用 apiErrorMessage() 取展示文案,
 *   不再各自解析 `error.data.message`。
 * - 服务端渲染需要透传 Cookie 的调用继续使用 useRequestFetch(),
 *   错误文案同样交给 apiErrorMessage()。
 */

export interface ApiErrorInit {
  statusCode?: number
  statusMessage?: string
  payload?: unknown
}

export class ApiError extends Error {
  readonly statusCode: number
  readonly statusMessage: string
  readonly payload: unknown

  constructor(message: string, init: ApiErrorInit = {}) {
    super(message)
    this.name = 'ApiError'
    this.statusCode = init.statusCode ?? 500
    this.statusMessage = init.statusMessage ?? ''
    this.payload = init.payload
  }
}

function fieldMessage(body: unknown, keys: readonly string[]): string {
  if (!body || typeof body !== 'object') return ''
  const record = body as Record<string, unknown>
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return ''
}

function bodyMessage(body: unknown): string {
  if (typeof body === 'string') return body.trim()
  if (!body || typeof body !== 'object') return ''
  const nested = (body as Record<string, unknown>).data
  return (
    fieldMessage(body, ['message', 'statusMessage', 'error', 'detail']) ||
    (nested === body ? '' : bodyMessage(nested))
  )
}

/** 从任意失败对象里提取一条可展示的中文说明,永远返回字符串。 */
export function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) return error.message || fallback
  if (typeof error === 'string') return error.trim() || fallback
  if (error && typeof error === 'object') {
    const record = error as Record<string, unknown>
    // 只信任响应体里的说明:原生 Error('fetch failed') 之类不能直接展示给用户
    return (
      bodyMessage(record.data) ||
      fieldMessage(error, ['statusMessage', 'error', 'detail']) ||
      fallback
    )
  }
  return fallback
}

/** 提取 HTTP 状态码;拿不到时返回 0,便于界面区分网络失败。 */
export function apiStatusCode(error: unknown): number {
  if (error instanceof ApiError) return error.statusCode
  if (error && typeof error === 'object') {
    const record = error as Record<string, unknown>
    const status = record.statusCode ?? record.status
    if (typeof status === 'number') return status
  }
  return 0
}

function toApiError(error: unknown): ApiError {
  const statusCode = apiStatusCode(error) || 500
  const record = error && typeof error === 'object' ? (error as Record<string, unknown>) : null
  const statusMessage = typeof record?.statusMessage === 'string' ? record.statusMessage : ''
  return new ApiError(apiErrorMessage(error, '请求失败,请稍后重试'), {
    statusCode,
    statusMessage,
    payload: record?.data ?? error
  })
}

type FetchOptions = Parameters<typeof $fetch>[1]

/** 请求并返回完整响应体;默认不重试,失败抛 ApiError。 */
export async function apiFetch<T>(url: string, options: FetchOptions = {}): Promise<T> {
  try {
    return await $fetch<T>(url, { retry: false, ...options })
  } catch (error) {
    throw toApiError(error)
  }
}

/** 请求 `{ data: T }` 信封并解包,失败抛 ApiError。 */
export async function apiData<T>(url: string, options: FetchOptions = {}): Promise<T> {
  const body = await apiFetch<{ data?: T }>(url, options)
  if (!body || typeof body !== 'object' || !('data' in body)) {
    throw new ApiError('接口返回格式无效', { payload: body })
  }
  return body.data as T
}
