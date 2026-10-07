import { request } from './http'

/** 报告接口：生成（任务/时间范围）、台账、重试与受控 PDF 下载。 */

export type ReportStatus = 'GENERATING' | 'READY' | 'FAILED'

export interface Report {
  id: number
  scope_type: 'TASK' | 'DATE_RANGE'
  task_id: number | null
  range_start: string | null
  range_end: string | null
  status: ReportStatus
  error_code: string
  created_at: string
  finished_at: string | null
  download_url: string | null
}

export interface ReportListResult {
  items: Report[]
  total: number
  page: number
  page_size: number
}

export interface ReportListParams {
  scope_type?: string
  task_id?: number
  page?: number
  page_size?: number
}

export async function fetchReports(params: ReportListParams = {}): Promise<ReportListResult> {
  const qs = new URLSearchParams()
  if (params.scope_type) qs.set('scope_type', params.scope_type)
  if (params.task_id) qs.set('task_id', String(params.task_id))
  qs.set('page', String(params.page ?? 1))
  qs.set('page_size', String(params.page_size ?? 20))
  const data = await request<ReportListResult>(`/reports?${qs.toString()}`)
  return data
}

export async function createTaskReport(taskId: number): Promise<Report> {
  const data = await request<{ report: Report }>(`/reports/task/${taskId}`, { method: 'POST' })
  return data.report
}

export async function createRangeReport(start: string, end: string): Promise<Report> {
  const data = await request<{ report: Report }>('/reports/range', {
    method: 'POST',
    body: JSON.stringify({ start, end }),
  })
  return data.report
}

export async function retryReport(id: number): Promise<Report> {
  const data = await request<{ report: Report }>(`/reports/${id}/retry`, { method: 'POST' })
  return data.report
}

/** 受控 PDF 下载：带 token 拉取后转 blob URL（禁止直接暴露存储路径）。 */
export async function loadReportBlob(id: number): Promise<{ blobUrl: string; fileName: string; size: number }> {
  const token = localStorage.getItem('city_wall_token') ?? ''
  const resp = await fetch(`${import.meta.env.VITE_API_BASE ?? '/api/v1'}/reports/${id}/file`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!resp.ok) throw new Error(`download report ${resp.status}`)
  const blob = await resp.blob()
  const disposition = resp.headers.get('Content-Disposition') ?? ''
  const m = disposition.match(/filename="?([^";]+)"?/)
  return {
    blobUrl: URL.createObjectURL(blob),
    fileName: m?.[1] ?? `report-${id}.pdf`,
    size: blob.size,
  }
}
