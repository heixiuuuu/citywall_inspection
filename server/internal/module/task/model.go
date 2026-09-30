// Package task 承载巡检任务与任务点领域模型。
// 字段与 04 数据模型文档保持一致；状态转移规则由 Service 校验，Repository 只做数据访问。
package task

import "time"

// 任务状态（04 文档 7.1）。
const (
	StatusDraft         = "DRAFT"          // 草稿
	StatusAssigned      = "ASSIGNED"       // 待执行
	StatusInProgress    = "IN_PROGRESS"    // 进行中
	StatusPendingReview = "PENDING_REVIEW" // 待审核
	StatusReturned      = "RETURNED"       // 退回补正
	StatusCompleted     = "COMPLETED"      // 已完成（终态）
	StatusCancelled     = "CANCELLED"      // 已取消（终态）
)

// Task 巡检任务。revision 用于乐观锁并发校验。
type Task struct {
	ID           int64      `gorm:"primaryKey;column:id" json:"id"`
	Title        string     `gorm:"column:title" json:"title"`
	AssigneeID   int64      `gorm:"column:assignee_id;index" json:"assignee_id"`
	DueAt        *time.Time `gorm:"column:due_at" json:"due_at"`
	Description  string     `gorm:"column:description" json:"description"`
	Status       string     `gorm:"column:status" json:"status"`
	Revision     int64      `gorm:"column:revision" json:"revision"`
	DispatchedAt *time.Time `gorm:"column:dispatched_at" json:"dispatched_at"`
	CompletedAt  *time.Time `gorm:"column:completed_at" json:"completed_at"`
	CancelledAt  *time.Time `gorm:"column:cancelled_at" json:"cancelled_at"`
	CancelReason string     `gorm:"column:cancel_reason" json:"cancel_reason"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// TaskPoint 任务点：草稿阶段仅登记 point_id 与序号，派发时写入点位快照，
// 后续修改基础点位不改旧任务。
type TaskPoint struct {
	ID                   int64     `gorm:"primaryKey;column:id" json:"id"`
	TaskID               int64     `gorm:"column:task_id;index:idx_task_seq" json:"task_id"`
	PointID              int64     `gorm:"column:point_id" json:"point_id"`
	SequenceNo           int       `gorm:"column:sequence_no;index:idx_task_seq" json:"sequence_no"`
	SnapshotName         *string   `gorm:"column:snapshot_name" json:"snapshot_name"`
	SnapshotSegmentID    *int64    `gorm:"column:snapshot_segment_id" json:"snapshot_segment_id"`
	SnapshotLatitude     *float64  `gorm:"column:snapshot_latitude" json:"snapshot_latitude"`
	SnapshotLongitude    *float64  `gorm:"column:snapshot_longitude" json:"snapshot_longitude"`
	SnapshotRadiusM      *float64  `gorm:"column:snapshot_radius_m" json:"snapshot_radius_m"`
	SnapshotLocationHint *string   `gorm:"column:snapshot_location_hint" json:"snapshot_location_hint"`
	CreatedAt            time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 显式表名。
func (Task) TableName() string      { return "tasks" }
func (TaskPoint) TableName() string { return "task_points" }
