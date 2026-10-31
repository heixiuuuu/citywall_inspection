// Package issue 承载病害问题的受理、指派、处置与核验领域模型。
// 问题与任务生命周期独立：任务完成时问题可保持开放；来源经 issue_observations 追溯。
package issue

import "time"

// 问题状态（04 文档 7.4）。
const (
	StatusUnassigned          = "UNASSIGNED"           // 待分配
	StatusOpen                = "OPEN"                 // 已分配待跟进
	StatusInProgress          = "IN_PROGRESS"          // 处理中
	StatusMonitoring          = "MONITORING"           // 观察中
	StatusPendingVerification = "PENDING_VERIFICATION" // 待核验
	StatusClosed              = "CLOSED"               // 已关闭
)

// 人工业务优先级（非模型推断的结构安全等级）。
const (
	PriorityWatch   = "WATCH"   // 关注
	PriorityProcess = "PROCESS" // 需处理
	PriorityUrgent  = "URGENT"  // 需尽快核实
)

// 处置动作类型（只追加，不改写历史正文）。
const (
	ActionAdvice        = "ADVICE"         // 处理建议
	ActionProgress      = "PROGRESS"       // 处理进展
	ActionObserve       = "OBSERVE"        // 观察记录
	ActionRequestVerify = "REQUEST_VERIFY" // 申请核验
	ActionVerifyPass    = "VERIFY_PASS"    // 核验通过
	ActionVerifyReturn  = "VERIFY_RETURN"  // 核验退回
	ActionReopen        = "REOPEN"         // 重新打开
	ActionAssign        = "ASSIGN"         // 指派/改派
)

// Issue 病害问题。task_id 不存在于本表：来源通过 issue_observations 关联现场记录追溯。
type Issue struct {
	AssigneeName   string     `gorm:"->;-:migration;column:assignee_name" json:"assignee_name"`
	SegmentName    string     `gorm:"->;-:migration;column:segment_name" json:"segment_name"`
	PointName      string     `gorm:"->;-:migration;column:point_name" json:"point_name"`
	NextFollowUpAt *time.Time `gorm:"->;-:migration;column:next_follow_up_at" json:"next_follow_up_at"`
	ID             int64      `gorm:"primaryKey;column:id" json:"id"`
	Title          string     `gorm:"column:title" json:"title"`
	SegmentID      *int64     `gorm:"column:segment_id" json:"segment_id"`
	PointID        *int64     `gorm:"column:point_id" json:"point_id"`
	LocationText   string     `gorm:"column:location_text" json:"location_text"`
	Description    string     `gorm:"column:description" json:"description"`
	PriorityCode   string     `gorm:"column:priority_code" json:"priority_code"`
	PriorityReason string     `gorm:"column:priority_reason" json:"priority_reason"`
	Status         string     `gorm:"column:status" json:"status"`
	AssigneeID     *int64     `gorm:"column:assignee_id" json:"assignee_id"`
	Revision       int64      `gorm:"column:revision" json:"revision"`
	OpenedAt       time.Time  `gorm:"column:opened_at" json:"opened_at"`
	ClosedAt       *time.Time `gorm:"column:closed_at" json:"closed_at"`
	CloseReason    string     `gorm:"column:close_reason" json:"close_reason"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// IssueObservation 问题与现场记录关联（一次审核可关联多个 Issue；一个问题可关联多个记录）。
type IssueObservation struct {
	IssueID          int64     `gorm:"primaryKey;column:issue_id" json:"issue_id"`
	ObservationID    int64     `gorm:"primaryKey;column:observation_id" json:"observation_id"`
	ReviewDecisionID int64     `gorm:"column:review_decision_id" json:"review_decision_id"`
	ConclusionItemID *int64    `gorm:"column:conclusion_item_id" json:"conclusion_item_id"`
	LinkReason       string    `gorm:"column:link_reason" json:"link_reason"`
	LinkedBy         int64     `gorm:"column:linked_by" json:"linked_by"`
	LinkedAt         time.Time `gorm:"column:linked_at" json:"linked_at"`
}

// IssueAction 处置动作（建议/进展/观察/申请核验/核验通过/退回/重开/指派）。
type IssueAction struct {
	ActorName         string        `gorm:"->;-:migration;column:actor_name" json:"actor_name"`
	Images            []ActionImage `gorm:"-" json:"images"`
	ID                int64         `gorm:"primaryKey;column:id" json:"id"`
	IssueID           int64         `gorm:"column:issue_id" json:"issue_id"`
	ActorID           int64         `gorm:"column:actor_id" json:"actor_id"`
	ActionType        string        `gorm:"column:action_type" json:"action_type"`
	Content           string        `gorm:"column:content" json:"content"`
	PlannedFollowUpAt *time.Time    `gorm:"column:planned_follow_up_at" json:"planned_follow_up_at"`
	CreatedAt         time.Time     `gorm:"column:created_at" json:"created_at"`
}

// IssueActionImage 处置动作附件（照片证据；读取时校验 Issue 权限）。
type IssueActionImage struct {
	IssueActionID int64 `gorm:"primaryKey;column:issue_action_id" json:"issue_action_id"`
	ImageID       int64 `gorm:"primaryKey;column:image_id" json:"image_id"`
}

// TableName 显式表名。
func (Issue) TableName() string            { return "issues" }
func (IssueObservation) TableName() string { return "issue_observations" }
func (IssueAction) TableName() string      { return "issue_actions" }
func (IssueActionImage) TableName() string { return "issue_action_images" }
