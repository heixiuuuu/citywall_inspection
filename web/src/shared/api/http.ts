/**
 * 统一请求函数：附带 token 与 X-Request-Id；
 * 对 401、403、409 与 5xx 做不同反馈，409 后重新拉取记录供用户判断。
 */

const BASE_URL = import.meta.env.VITE_API_BASE ?? '/api/v1'

export interface ApiError {
  code: string
  message: string
  details?: Record<string, unknown>
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (!(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  const token = localStorage.getItem('city_wall_token')
  if (token) headers.set('Authorization', `Bearer ${token}`)
  headers.set('X-Request-Id', genRequestId())

  const resp = await fetch(`${BASE_URL}${path}`, { ...init, headers })
  const body = await resp.json().catch(() => null)

  if (!resp.ok) {
    const err: ApiError = body?.error ?? { code: 'UNKNOWN', message: `HTTP ${resp.status}` }
    // TODO: 按 code 分类提示；409 提示用户重新拉取记录后决定是否重试。
    throw new ApiRequestError(resp.status, err)
  }
  return body?.data as T
}

export class ApiRequestError extends Error {
  constructor(
    readonly status: number,
    readonly error: ApiError,
  ) {
    super(error.message)
    this.name = 'ApiRequestError'
  }
}

function genRequestId(): string {
  return `req_${Date.now().toString(36)}${Math.random().toString(36).slice(2, 10)}`
}
