import { request } from './http'

export interface Assignee { id: number; display_name: string }
export async function fetchAssignees(): Promise<Assignee[]> {
  return (await request<{items: Assignee[]}>('/tasks/assignees')).items
}

/** 任务接口（与 server/api/openapi.yaml 契约对齐）。 */

export type TaskStatus =
  | 'DRAFT'
  | 'ASSIGNED'
  | 'IN_PROGRESS'
  | 'PENDING_REVIEW'
  | 'RETURNED'
  | 'COMPLETED'
  | 'CANCELLED'

export interface Task {
  id: number
  title: string
  assignee_id: number
  due_at: string | null
  description: string | null
  status: TaskStatus
  revision: number
  dispatched_at: string | null
  completed_at: string | null
  cancelled_at: string | null
  cancel_reason: string | null
}

export interface TaskPoint {
  point_name: string
  observation_id: number | null
  observation_reporter_id: number | null
  observation_status: string
  observation_outcome: string
  id: number
  task_id: number
  point_id: number
  sequence_no: number
  snapshot_name: string | null
  snapshot_segment_id: number | null
  snapshot_latitude: number | null
  snapshot_longitude: number | null
  snapshot_radius_m: number | null
  snapshot_location_hint: string | null
}

export interface CreateTaskPayload {
  title: string
  assignee_id: number
  due_at?: string
  description?: string
  point_ids: number[]
}

export interface TaskListParams {
  keyword?: string
  overdue?: boolean
  status?: string
  page?: number
  page_size?: number
}

export interface TaskListResult {
  tasks: Task[]
  total: number
  page: number
  page_size: number
}

export async function fetchTasks(params: TaskListParams = {}): Promise<TaskListResult> {
  const qs = new URLSearchParams()
  if (params.keyword) qs.set('keyword', params.keyword)
  if (params.overdue) qs.set('overdue', '1')
  if (params.status) qs.set('status', params.status)
  qs.set('page', String(params.page ?? 1))
  qs.set('page_size', String(params.page_size ?? 20))
  const data = await request<TaskListResult>(`/tasks?${qs.toString()}`)
  return data
}

export async function createTask(payload: CreateTaskPayload): Promise<Task> {
  const data = await request<{ task: Task }>('/tasks', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
  return data.task
}

export async function fetchTask(id: number): Promise<{ task: Task; points: TaskPoint[] }> {
  return request(`/tasks/${id}`)
}

export async function dispatchTask(id: number, revision: number): Promise<Task> {
  const data = await request<{ task: Task }>(`/tasks/${id}/dispatch`, {
    method: 'POST',
    body: JSON.stringify({ revision }),
  })
  return data.task
}

export async function completeTask(id: number, revision: number, finalNote?: string): Promise<Task> {
  const data = await request<{ task: Task }>(`/tasks/${id}/complete`, {
    method: 'POST',
    body: JSON.stringify({ revision, final_note: finalNote ?? '' }),
  })
  return data.task
}

export async function cancelTask(id: number, revision: number, reason: string): Promise<Task> {
  const data = await request<{ task: Task }>(`/tasks/${id}/cancel`, {
    method: 'POST',
    body: JSON.stringify({ revision, reason }),
  })
  return data.task
}
