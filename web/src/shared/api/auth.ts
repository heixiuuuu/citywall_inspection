import { request } from './http'

/** 身份与登录接口。 */

export interface User {
  id: number
  username: string | null
  wechat_openid: string | null
  display_name: string
  status: 'ACTIVE' | 'DISABLED'
}

export interface Session {
  token: string
  user: User
  roles: string[]
}

export async function login(username: string, password: string): Promise<Session> {
  return request<Session>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

export async function fetchMe(): Promise<{ user: User; roles: string[] }> {
  return request('/me')
}
