// Package report 承载报告领域模型：任务报告与时间范围报告。
// 报告在生成时刻保存数据快照（snapshot_json），后续问题/记录状态变化不修改旧报告。
package report

import "time"

// 报告范围。
const (
	ScopeTask      = "TASK"      // 单任务报告
	ScopeDateRange = "DATE_RANGE" // 时间范围报告
)

// 报告状态。
const (
	StatusGenerating = "GENERATING"
	StatusReady      = "READY"
	StatusFailed     = "FAILED"
)

// Report 报告记录；snapshot_json 为生成时刻的结构化数据快照。
type Report struct {
	ID            int64      `gorm:"primaryKey;column:id" json:"id"`
	ScopeType     string     `gorm:"column:scope_type" json:"scope_type"`
	TaskID        *int64     `gorm:"column:task_id" json:"task_id"`
	RangeStart    *time.Time `gorm:"column:range_start" json:"range_start"`
	RangeEnd      *time.Time `gorm:"column:range_end" json:"range_end"`
	GeneratedBy   int64      `gorm:"column:generated_by" json:"generated_by"`
	SnapshotAt    time.Time  `gorm:"column:snapshot_at" json:"snapshot_at"`
	SnapshotJSON  string     `gorm:"column:snapshot_json;type:mediumtext" json:"snapshot_json"`
	FileKey       *string    `gorm:"column:file_key" json:"file_key"`
	Status        string     `gorm:"column:status" json:"status"`
	ErrorCode     string     `gorm:"column:error_code" json:"error_code"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
	FinishedAt    *time.Time `gorm:"column:finished_at" json:"finished_at"`
}

// TableName 显式表名。
func (Report) TableName() string { return "reports" }

// FileInfo PDF 下载信息。
type FileInfo struct {
	FileKey    string
	MediaType  string
	ByteSize   int64
}
