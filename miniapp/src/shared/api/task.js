import { request } from './request'

/** 任务相关接口（与 server/api/openapi.yaml 契约对齐）。 */

/** GET /tasks 本人或授权范围任务列表。 */
export function getTasks() {
  return request('/tasks')
}

/** POST /tasks 创建任务草稿（管理员）。 */
export function createTask(payload) {
  return request('/tasks', { method: 'POST', data: payload })
}
