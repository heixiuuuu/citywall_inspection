import { request } from './http'

/** 审计事件接口（与 server/api/openapi.yaml 契约对齐，仅管理员）。 */

export interface AuditEvent {
  id: number
  entity_type: string
  entity_id: number
  action: string
  actor_id: number
  old_status: string | null
  new_status: string | null
  revision: number | null
  reason: string | null
  created_at: string
}

export interface AuditListParams {
  entity_type?: string
  entity_id?: string
  actor_id?: string
  page?: number
  page_size?: number
}

export interface AuditListResult {
  events: AuditEvent[]
  total: number
  page: number
  page_size: number
}

export async function fetchAuditEvents(params: AuditListParams = {}): Promise<AuditListResult> {
  const qs = new URLSearchParams()
  if (params.entity_type) qs.set('entity_type', params.entity_type)
  if (params.entity_id) qs.set('entity_id', params.entity_id)
  if (params.actor_id) qs.set('actor_id', params.actor_id)
  qs.set('page', String(params.page ?? 1))
  qs.set('page_size', String(params.page_size ?? 50))
  return request<AuditListResult>(`/audit/events?${qs.toString()}`)
}
