// Package audit 记录不可修改的操作历史（状态变化与关键更正）。
// 不存 token、密码与图片正文。
package audit

import (
	"context"
	"time"
)

// Event 一条审计事件。
type Event struct {
	ID        int64      `gorm:"primaryKey;column:id" json:"id"`
	EntityType string    `gorm:"column:entity_type;index" json:"entity_type"` // task/observation/issue/...
	EntityID  int64      `gorm:"column:entity_id;index" json:"entity_id"`
	Action    string     `gorm:"column:action" json:"action"` // dispatch/submit/review/complete/...
	ActorID   int64      `gorm:"column:actor_id;index" json:"actor_id"`
	OldStatus string     `gorm:"column:old_status" json:"old_status"`
	NewStatus string     `gorm:"column:new_status" json:"new_status"`
	Revision  *int64     `gorm:"column:revision" json:"revision"`
	Reason    string     `gorm:"column:reason" json:"reason"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
}

// TableName 显式表名。
func (Event) TableName() string { return "audit_events" }

// Repository 审计数据访问。
type Repository interface {
	Insert(ctx context.Context, e *Event) error
}

// Recorder 无数据库时（dev 降级）的空实现。
type Recorder interface {
	Record(ctx context.Context, e *Event)
}
