import { request } from './http'

/** 巡检范围（区段/点位）。 */

export interface WallSegment {
  id: number
  code: string
  name: string
  description: string | null
  active: boolean
}

export interface InspectionPoint {
  id: number
  segment_id: number
  code: string
  name: string
  latitude: number
  longitude: number
  radius_m: number | null
  location_hint: string | null
  active: boolean
}

export interface CreatePointPayload {
  segment_id: number
  code: string
  name: string
  latitude: number
  longitude: number
  radius_m?: number
  location_hint?: string
  active?: boolean
}

export async function fetchSegments(includeInactive = false): Promise<WallSegment[]> {
  const data = await request<{ items: WallSegment[] }>(`/wall-segments?include_inactive=${includeInactive}`)
  return data.items
}

export async function fetchPoints(includeInactive = false): Promise<InspectionPoint[]> {
  const data = await request<{ items: InspectionPoint[] }>(`/inspection-points?include_inactive=${includeInactive}`)
  return data.items
}

export async function createPoint(payload: CreatePointPayload): Promise<InspectionPoint> {
  const data = await request<{ point: InspectionPoint }>('/inspection-points', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
  return data.point
}

export async function updatePoint(id: number, payload: CreatePointPayload): Promise<InspectionPoint> {
  return (await request<{ point: InspectionPoint }>(`/inspection-points/${id}`, { method: 'PUT', body: JSON.stringify(payload) })).point
}
export async function saveSegment(id: number | null, payload: { code: string; name: string; description: string; active: boolean }): Promise<WallSegment> {
  return (await request<{ segment: WallSegment }>(id ? `/wall-segments/${id}` : '/wall-segments', { method: id ? 'PUT' : 'POST', body: JSON.stringify(payload) })).segment
}
