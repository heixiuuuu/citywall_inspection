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

  let resp: Response
  try { resp = await fetch(`${BASE_URL}${path}`, { ...init, headers }) }
  catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error
    throw new Error('暂时无法连接服务，请检查网络后重试。')
  }
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
  if (!body || !('data' in body)) throw new Error('服务返回的数据不完整，请稍后重试。')
  return body.data as T
}

export class ApiRequestError extends Error {
  constructor(
    readonly status: number,
    readonly error: ApiError,
  ) {
    super(apiErrorMessage(status, error))
    this.name = 'ApiRequestError'
  }
}

function apiErrorMessage(status: number, error: ApiError): string {
  if (/[\u4e00-\u9fff]/.test(error.message)) return error.message
  const reasons: Record<string, string> = {
    'cannot review own record': '不能审核本人上报的记录，请由其他审核员处理。',
    'cannot verify own submitted verification': '不能核验本人提交的处置结果，请由其他审核员处理。',
    'human conclusion required': '请先填写人工结论，再提交记录。',
    'at least one overview image required': '请上传至少一张现场概览照片。',
    'some inspection points not completed': '仍有点位未完成提交或审核，请在任务详情中检查。',
    'assignee must be an active': '所选责任人当前不可用，请重新选择。',
    'reason required': '请填写操作说明后继续。',
    'return reason required': '请填写具体的退回或驳回原因。',
  }
  for (const [text, message] of Object.entries(reasons)) if (error.message.includes(text)) return message
  const messages: Record<string, string> = {
    INVALID_CREDENTIALS: '账号或密码不正确，请重新输入。', USER_DISABLED: '当前账号已停用，请联系管理员。',
    UNAUTHORIZED: '登录已失效，请重新登录。', FORBIDDEN: '当前账号没有此操作权限，请联系管理员。',
    CONFLICT: '记录状态已变化，请刷新后确认当前内容。', NOT_FOUND: '未找到这项数据，请刷新列表后重试。',
    VALIDATION_ERROR: '提交内容不完整或不符合要求，请检查填写内容。',
    DB_UNAVAILABLE: '业务服务暂时不可用，请稍后重试。', INTERNAL_ERROR: '操作暂时未能完成，请稍后重试。',
    IMAGE_TOO_LARGE: '照片过大，请选择较小的照片。', UNSUPPORTED_MEDIA_TYPE: '文件格式不支持，请使用 JPEG、PNG 或 WebP 照片。',
    INVALID_IMAGE: '照片无法读取，请重新选择。', NOT_READY: '报告尚未生成完成，请稍后刷新。',
  }
  return messages[error.code] ?? (status >= 500 ? '服务暂时不可用，请稍后重试。' : '操作未完成，请刷新后重试。')
}

function genRequestId(): string {
  return `req_${Date.now().toString(36)}${Math.random().toString(36).slice(2, 10)}`
}
