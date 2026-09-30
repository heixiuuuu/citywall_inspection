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
  inspection_outcome: string | null
  status: ObservationStatus
  revision: number
  submitted_at: string | null
  reviewed_at: string | null
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
  const resp = await fetch(`${import.meta.env.VITE_API_BASE ?? '/api/v1'}/images/${imageId}/content`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!resp.ok) throw new Error(`load image ${resp.status}`)
  const blob = await resp.blob()
  return URL.createObjectURL(blob)
}

export type { Task, TaskPoint, CreateTaskPayload }
