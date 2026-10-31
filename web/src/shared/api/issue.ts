/**
 * 问题台账与处置 API（对应 Go module/issue 与 /issues、/observations/:id/admit）。
 */
import { request } from './http'

export interface Issue {
	assignee_name: string
	segment_name: string
	point_name: string
	next_follow_up_at: string | null
  id: number
  title: string
  segment_id: number | null
  point_id: number | null
  location_text: string
  description: string
  priority_code: 'WATCH' | 'PROCESS' | 'URGENT'
  priority_reason: string
  status: string
  assignee_id: number | null
  revision: number
  opened_at: string
  closed_at: string | null
  close_reason: string
}

export interface IssueAction {
	actor_name: string
  images: IssueActionImage[]
  id: number
  issue_id: number
  actor_id: number
  action_type: string
  content: string
  planned_follow_up_at: string | null
  created_at: string
}

export interface IssueActionImage {
  id: number
  media_type: string
  byte_size: number
  width: number
  height: number
  url: string
}

export function uploadIssueActionImage(issueId: number, actionId: number, file: File) {
  const body = new FormData()
  body.append('image', file)
  return request<{ image: IssueActionImage }>(`/issues/${issueId}/actions/${actionId}/images`, { method: 'POST', body })
}

export interface IssueObservationLink {
  observation_id: number
  source?: string
  location_text?: string
  link_reason: string
  linked_at: string
}

export interface IssueDetail {
  issue: Issue
  observations: IssueObservationLink[]
  actions: IssueAction[]
}

export interface NewIssueInput {
  title: string
  segment_id?: number | null
  point_id?: number | null
  location_text?: string
  description?: string
  priority_code: 'WATCH' | 'PROCESS' | 'URGENT'
  priority_reason?: string
}

export interface AdmitResult {
  created_issue_ids: number[]
  linked_issue_ids: number[]
  no_issue_reason: string
  already_admitted: boolean
}

export interface IssueListParams { status?: string; keyword?: string; priority_code?: string; only_mine?: boolean; active_only?: boolean; follow_up_due?: boolean; page?: number; page_size?: number }
export function fetchIssues(params: IssueListParams = {}) {
  const q = new URLSearchParams()
  if (params.keyword) q.set('keyword', params.keyword)
  if (params.priority_code) q.set('priority_code', params.priority_code)
  if (params.only_mine) q.set('only_mine', '1')
  if (params.active_only) q.set('active_only', '1')
  if (params.follow_up_due) q.set('follow_up_due', '1')
  if (params.status) q.set('status', params.status)
  if (params.page) q.set('page', String(params.page))
  if (params.page_size) q.set('page_size', String(params.page_size))
  return request<{ items: Issue[]; total: number; page: number; page_size: number }>(`/issues?${q.toString()}`)
}

export function fetchIssue(id: number) {
  return request<IssueDetail>(`/issues/${id}`)
}

export interface IssueAssignee { id: number; display_name: string }
export async function fetchIssueAssignees() {
  const result = await request<{ items: IssueAssignee[] }>('/issues/assignees')
  return result.items
}

export function assignIssue(id: number, revision: number, assigneeId: number, reason?: string) {
  return request<{ issue: Issue }>(`/issues/${id}/assign`, {
    method: 'POST',
    body: JSON.stringify({ revision, assignee_id: assigneeId, reason: reason || undefined }),
  })
}

export function addIssueAction(
  id: number,
  revision: number,
  input: { action_type: string; content: string; planned_follow_up_at?: string },
) {
  return request<{ issue: Issue }>(`/issues/${id}/actions`, {
    method: 'POST',
    body: JSON.stringify({ revision, ...input }),
  })
}

export function requestVerification(id: number, revision: number, content: string) {
  return request<{ issue: Issue }>(`/issues/${id}/request-verification`, {
    method: 'POST',
    body: JSON.stringify({ revision, content }),
  })
}

export function verifyIssue(id: number, revision: number, decision: 'PASS' | 'RETURN', reason?: string) {
  return request<{ issue: Issue }>(`/issues/${id}/verify`, {
    method: 'POST',
    body: JSON.stringify({ revision, decision, reason: reason || undefined }),
  })
}

export function reopenIssue(id: number, revision: number, reason: string) {
  return request<{ issue: Issue }>(`/issues/${id}/reopen`, {
    method: 'POST',
    body: JSON.stringify({ revision, reason }),
  })
}

export function admitObservation(obsId: number, input: { new_issues?: NewIssueInput[]; link_issue_ids?: number[]; no_issue_reason?: string }) {
  return request<{ admit: AdmitResult }>(`/observations/${obsId}/admit`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export const ISSUE_STATUS: Record<string, { label: string; type: 'info' | 'primary' | 'warning' | 'danger' | 'success' }> = {
  UNASSIGNED: { label: '待分配', type: 'info' },
  OPEN: { label: '已分配', type: 'primary' },
  IN_PROGRESS: { label: '处理中', type: 'warning' },
  MONITORING: { label: '观察中', type: 'warning' },
  PENDING_VERIFICATION: { label: '待核验', type: 'danger' },
  CLOSED: { label: '已关闭', type: 'success' },
}

export const ISSUE_PRIORITY: Record<string, { label: string; type: 'info' | 'primary' | 'warning' | 'danger' }> = {
  WATCH: { label: '关注', type: 'info' },
  PROCESS: { label: '需处理', type: 'warning' },
  URGENT: { label: '尽快核实', type: 'danger' },
}

export const ACTION_TYPE_TEXT: Record<string, string> = {
  ADVICE: '处理建议',
  PROGRESS: '处理进展',
  OBSERVE: '观察记录',
  REQUEST_VERIFY: '申请核验',
  VERIFY_PASS: '核验通过',
  VERIFY_RETURN: '核验退回',
  REOPEN: '重新打开',
  ASSIGN: '指派',
}
