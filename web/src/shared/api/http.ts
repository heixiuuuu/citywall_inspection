import { router } from '@/router'

const BASE_URL = import.meta.env.VITE_API_BASE ?? '/api/v1'

export interface ApiError {
  code: string
  message: string
  details?: Record<string, unknown>
}

export async function authenticatedFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  if (!(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  const token = localStorage.getItem('city_wall_token')
  if (token) headers.set('Authorization', `Bearer ${token}`)
  headers.set('X-Request-Id', genRequestId())

  const resp = await fetch(`${BASE_URL}${path}`, { ...init, headers })
  if (resp.status === 401 && token && !path.startsWith('/auth/') && token === localStorage.getItem('city_wall_token')) {
    localStorage.removeItem('city_wall_token')
    localStorage.removeItem('city_wall_roles')
    localStorage.removeItem('city_wall_user')
    const route = router.currentRoute.value
    if (route.name !== 'login') {
      void router.replace({ name: 'login', query: { expired: '1', redirect: route.fullPath } })
    }
  }
  return resp
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const resp = await authenticatedFetch(path, init)
  const body = await resp.json().catch(() => null)

  if (!resp.ok) {
    const err: ApiError = body?.error ?? { code: 'UNKNOWN', message: `HTTP ${resp.status}` }
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
