/**
 * 与公开 API 对齐的类型（03 文档 6.1 命名约定）。
 * 字段与 server/api/openapi.yaml、04 数据模型保持一致。
 */

/** 任务状态枚举。 */
export type TaskStatus =
  | 'DRAFT'
  | 'ASSIGNED'
  | 'IN_PROGRESS'
  | 'PENDING_REVIEW'
  | 'RETURNED'
  | 'COMPLETED'
  | 'CANCELLED'

export interface Task {
  id: string
  title: string
  assignee_id: string
  status: TaskStatus
  due_at?: string
  description?: string
  revision: number
}

/** 现场记录来源。 */
export type ObservationSource = 'TASK' | 'VOLUNTARY'

/** 现场结论。 */
export type InspectionOutcome = 'NORMAL' | 'SUSPECTED' | 'UNDETERMINED' | 'UNREACHABLE'

export interface Observation {
  id: string
  source: ObservationSource
  task_point_id?: string
  location_text: string
  inspection_outcome?: InspectionOutcome
  status: 'DRAFT' | 'SUBMITTED' | 'RETURNED' | 'ACCEPTED' | 'REJECTED'
  revision: number
}

/** 通用响应包装。 */
export interface ApiEnvelope<T> {
  data: T
  request_id: string
}
