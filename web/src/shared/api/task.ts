import { request } from './http'

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
}

export interface TaskPoint {
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

export async function fetchTasks(): Promise<Task[]> {
  const data = await request<{ tasks: Task[] }>('/tasks')
  return data.tasks
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
