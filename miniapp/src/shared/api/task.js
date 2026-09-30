import { request } from './request'

/** 任务相关接口（与 server/api/openapi.yaml 契约对齐）。 */

/** POST /auth/wechat-login 微信身份登录（MVP 固定测试 code）。 */
export function wechatLogin(code) {
  return request('/auth/wechat-login', { method: 'POST', data: { code } })
}

/** GET /tasks 本人任务列表。 */
export function getTasks() {
  return request('/tasks')
}

/** GET /tasks/{id} 任务详情与点位。 */
export function getTask(id) {
  return request(`/tasks/${id}`)
}

/** POST /tasks/{id}/submit 提交整任务（负责人）。 */
export function submitTask(id, revision) {
  return request(`/tasks/${id}/submit`, { method: 'POST', data: { revision } })
}
