import { request } from './http'
import type { Task, TaskPoint, CreateTaskPayload } from './task'

/** 现场记录与审核接口。 */

export type ObservationStatus = 'DRAFT' | 'SUBMITTED' | 'RETURNED' | 'ACCEPTED' | 'REJECTED'

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
  captured_at: string | null
  inspection_outcome: string | null
  status: ObservationStatus
  revision: number
  submitted_at: string | null
  reviewed_at: string | null
  created_at: string
}

export interface ConclusionRevision {
  id: number
  observation_id: number
  revision_no: number
  outcome: string
  description: string
  priority_code: string | null
  author_id: number
}

export interface Detection {
  id: number; attempt_id: number; class_code: string; confidence: number
  x1: number; y1: number; x2: number; y2: number
}
export interface RecognitionAttempt {
  id: number; image_id: number; attempt_no: number; status: string
  model_version: string; error_code: string; analysis_json: string
}
export interface VisionAnalysis {
  status: 'SUCCEEDED' | 'DISABLED' | 'FAILED'; model: string; description: string
  possible_causes: string[]; recommendations: string[]; error_code: string
}
export interface Evidence {
  observation: Observation
  attempts: RecognitionAttempt[]
  detections: Detection[]
  conclusions: ConclusionRevision[]
  items: { id: number; conclusion_revision_id: number; image_id: number; decision: string; final_class_code: string; description: string }[]
  reviews: { id: number; decision: string; reason: string; disposition: string; created_at: string }[]
}
export async function fetchEvidence(id: number): Promise<Evidence> {
  return (await request<{ evidence: Evidence }>(`/observations/${id}`)).evidence
}

export type ReviewDecision = 'ACCEPT' | 'RETURN' | 'REJECT'

export interface ReviewPayload {
  revision: number
  decision: ReviewDecision
  reason?: string
  disposition?: string
}

/** 记录图片元数据（GET /observations/{id}/images）。 */
export interface ObservationImageMeta {
  id: number
  role: 'OVERVIEW' | 'DETAIL'
  media_type: string
  byte_size: number
  width: number
  height: number
  url: string
}

/** GET /reviews 待审核记录列表。 */
export async function fetchPendingReviews(): Promise<Observation[]> {
  const data = await request<{ items: Observation[] }>('/reviews')
  return data.items
}

/** GET /observations/{id}/images 记录图片元数据列表。 */
export async function fetchObservationImages(id: number): Promise<ObservationImageMeta[]> {
  const data = await request<{ images: ObservationImageMeta[] }>(`/observations/${id}/images`)
  return data.images
}

/** POST /observations/{id}/review 审核记录。 */
export async function reviewObservation(id: number, payload: ReviewPayload): Promise<Observation> {
  const data = await request<{ observation: Observation }>(`/observations/${id}/review`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
  return data.observation
}

/** 受控图片内容：带 token 拉取后转 blob URL（禁止直接暴露存储路径）。 */
export async function loadImageBlob(imageId: number): Promise<string> {
  const token = localStorage.getItem('city_wall_token') ?? ''
  const resp = await fetch(`${import.meta.env.VITE_API_BASE ?? '/api/v1'}/images/${imageId}/preview`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!resp.ok) throw new Error(`load image ${resp.status}`)
  const blob = await resp.blob()
  return URL.createObjectURL(blob)
}

/** 现场记录检索（按任务/时间/点位/来源/审核状态过滤）。 */
export interface ObservationListParams {
  status?: string
  source?: string
  task_id?: number
  point_id?: number
  date_from?: string
  date_to?: string
  page?: number
  page_size?: number
}

export interface ObservationListResult {
  items: Observation[]
  total: number
  page: number
  page_size: number
}

export async function fetchObservations(params: ObservationListParams = {}): Promise<ObservationListResult> {
  const qs = new URLSearchParams()
  if (params.status) qs.set('status', params.status)
  if (params.source) qs.set('source', params.source)
  if (params.task_id) qs.set('task_id', String(params.task_id))
  if (params.point_id) qs.set('point_id', String(params.point_id))
  if (params.date_from) qs.set('date_from', params.date_from)
  if (params.date_to) qs.set('date_to', params.date_to)
  qs.set('page', String(params.page ?? 1))
  qs.set('page_size', String(params.page_size ?? 20))
  const data = await request<ObservationListResult>(`/observations?${qs.toString()}`)
  return data
}

export type { Task, TaskPoint, CreateTaskPayload }
