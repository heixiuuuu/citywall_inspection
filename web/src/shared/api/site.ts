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
}

export async function fetchSegments(): Promise<WallSegment[]> {
  const data = await request<{ items: WallSegment[] }>('/wall-segments')
  return data.items
}

export async function fetchPoints(): Promise<InspectionPoint[]> {
  const data = await request<{ items: InspectionPoint[] }>('/inspection-points')
  return data.items
}

export async function createPoint(payload: CreatePointPayload): Promise<InspectionPoint> {
  const data = await request<{ point: InspectionPoint }>('/inspection-points', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
  return data.point
}
