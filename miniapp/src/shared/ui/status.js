/**
 * 状态 → 文案/样式类 的统一映射（与 design/miniapp/ui-spec.md 状态色一致）。
 */
export function statusText(status) {
  return (
    {
      DRAFT: '草稿',
      ASSIGNED: '已派发',
      IN_PROGRESS: '采集中',
      PENDING_REVIEW: '待审核',
      RETURNED: '已退回',
      COMPLETED: '已完成',
      CANCELLED: '已取消',
      SUBMITTED: '待审核',
      ACCEPTED: '已通过',
      REJECTED: '已驳回',
    }[status] || status
  )
}

export function statusClass(status) {
  return (
    {
      DRAFT: 'draft',
      ASSIGNED: 'assigned',
      IN_PROGRESS: 'progress',
      PENDING_REVIEW: 'pending',
      RETURNED: 'bad',
      COMPLETED: 'done',
      CANCELLED: 'draft',
      SUBMITTED: 'pending',
      ACCEPTED: 'done',
      REJECTED: 'bad',
    }[status] || 'draft'
  )
}
