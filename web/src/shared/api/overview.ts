import { request } from './http'

export interface OverviewCount { code: string; total: number }
export interface Overview {
  generated_at: string
  tasks: OverviewCount[]
  records: OverviewCount[]
  issues: OverviewCount[]
  categories: OverviewCount[]
  overdue_tasks: number
  due_follow_ups: number
  pending_admission: number
  urgent_issues: number
  active_points: number
  missing_location: number
  low_accuracy: number
  failed_recognition: number
}

export async function fetchOverview() {
  return (await request<{ summary: Overview }>('/overview')).summary
}
