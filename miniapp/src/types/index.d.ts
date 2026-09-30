/**
 * 小程序端类型定义（与 server/api/openapi.yaml schemas 对齐）。
 * id 均为 number（服务端 BIGINT 序列化为 JSON 数字）。
 */

/** 任务状态（04 文档 7.1）。 */
export type TaskStatus =
  | 'DRAFT'
  | 'ASSIGNED'
  | 'IN_PROGRESS'
  | 'PENDING_REVIEW'
  | 'RETURNED'
  | 'COMPLETED'
  | 'CANCELLED'

/** 记录状态（04 文档 7.2）。 */
export type ObservationStatus = 'DRAFT' | 'SUBMITTED' | 'RETURNED' | 'ACCEPTED' | 'REJECTED'

/** 现场结论。 */
export type Outcome = 'NORMAL' | 'SUSPECTED' | 'UNDETERMINED' | 'UNREACHABLE'

/** 用户。 */
export interface User {
  id: number
  username: string | null
  wechat_openid: string | null
  display_name: string
  status: 'ACTIVE' | 'DISABLED'
}

/** 任务。 */
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

/** 任务点（派发后含快照）。 */
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

/** 现场记录。 */
export interface Observation {
  id: number
  source: 'TASK' | 'VOLUNTARY'
  task_point_id: number | null
  reporter_id: number
  segment_id: number | null
  point_id: number | null
  location_text: string
  latitude: number | null
  longitude: number | null
  accuracy_m: number | null
  distance_m: number | null
  captured_at: string | null
  inspection_outcome: Outcome | null
  status: ObservationStatus
  revision: number
  submitted_at: string | null
  reviewed_at: string | null
}

/** 图片元数据。 */
export interface Image {
  id: number
  storage_key: string
  sha256: string
  media_type: string
  byte_size: number
  width: number
  height: number
  uploaded_by: number
}

/** 识别尝试。 */
export interface RecognitionAttempt {
  id: number
  image_id: number
  attempt_no: number
  status: 'PENDING' | 'SUCCEEDED' | 'FAILED'
  model_version: string | null
  schema_version: string | null
  latency_ms: number | null
  error_code: string | null
}

/** 候选检测框（原图像素坐标）。 */
export interface Detection {
  id: number
  attempt_id: number
  detection_key: string
  class_code: string
  confidence: number
  x1: number
  y1: number
  x2: number
  y2: number
}

/** 人工结论版本。 */
export interface ConclusionRevision {
  id: number
  observation_id: number
  revision_no: number
  outcome: Outcome
  description: string
  priority_code: string | null
  author_id: number
}

/** 通用响应包装。 */
export interface ApiEnvelope<T> {
  data: T
  request_id: string
}
