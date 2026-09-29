import { request } from './http'

/** 与 server/api/openapi.yaml 契约对齐。 */

export interface Task {
  id: string
  title: string
  assignee_id: string
  status: string
  due_at?: string
  description?: string
  revision: number
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
