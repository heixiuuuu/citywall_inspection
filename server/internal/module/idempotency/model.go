// Package idempotency 提供跨请求幂等记录（落库，服务重启不丢失）。
// 用于派发、提交、审核、关闭、图片上传等重复执行会产生副作用的操作。
package idempotency

import (
	"errors"
	"time"
)

// 幂等记录状态。
const (
	StatusCompleted = "COMPLETED"
	StatusConflict  = "CONFLICT"
)

// Record 幂等记录（唯一键：actor_id + operation + idem_key）。
type Record struct {
	ID          int64      `gorm:"primaryKey;column:id"`
	ActorID     int64      `gorm:"column:actor_id"`
	Operation   string     `gorm:"column:operation"`
	IdemKey     string     `gorm:"column:idem_key"`
	RequestHash string     `gorm:"column:request_hash"`
	ResponseRef string     `gorm:"column:response_ref"`
	Status      string     `gorm:"column:status"`
	ExpiresAt   *time.Time `gorm:"column:expires_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
}

// TableName 显式表名。
func (Record) TableName() string { return "idempotency_records" }

// 领域错误。
var (
	// ErrKeyRequired 缺少幂等键。
	ErrKeyRequired = errors.New("idempotency: key required")
	// ErrRecordExists 同键记录已存在（唯一键冲突，并发重放）。
	ErrRecordExists = errors.New("idempotency: record exists")
	// ErrDBUnavailable 幂等存储不可用（降级：调用方忽略并继续）。
	ErrDBUnavailable = errors.New("idempotency: db unavailable")
)
