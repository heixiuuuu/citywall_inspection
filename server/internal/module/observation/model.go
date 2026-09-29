// Package observation 承载现场记录、图片、识别尝试与人工结论领域模型。
// 字段与 04 数据模型文档一致；识别结果始终是原始候选，人工结论与审核决定分别保存。
package observation

import "time"

// 记录来源。
const (
	SourceTask      = "TASK"      // 任务内检查，必须带 task_point_id
	SourceVoluntary = "VOLUNTARY" // 任务外主动上报，不带 task_point_id
)

// 现场结论（inspection_outcome）。
const (
	OutcomeNormal      = "NORMAL"
	OutcomeSuspected   = "SUSPECTED"
	OutcomeUndetermined = "UNDETERMINED"
	OutcomeUnreachable = "UNREACHABLE"
)

// 记录状态（04 文档 7.2）。
const (
	ObsDraft    = "DRAFT"
	ObsSubmitted = "SUBMITTED"
	ObsReturned = "RETURNED"
	ObsAccepted = "ACCEPTED"
	ObsRejected = "REJECTED"
)

// Image 图片元数据；storage_key 由服务端生成，目录不公开静态访问。
type Image struct {
	ID          int64     `gorm:"primaryKey;column:id" json:"id"`
	StorageKey  string    `gorm:"column:storage_key;uniqueIndex" json:"storage_key"`
	SHA256      string    `gorm:"column:sha256" json:"sha256"`
	MediaType   string    `gorm:"column:media_type" json:"media_type"`
	ByteSize    int64     `gorm:"column:byte_size" json:"byte_size"`
	Width       int       `gorm:"column:width" json:"width"`
	Height      int       `gorm:"column:height" json:"height"`
	CapturedAt  *time.Time `gorm:"column:captured_at" json:"captured_at"`
	UploadedBy  int64     `gorm:"column:uploaded_by" json:"uploaded_by"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

// Observation 现场记录。TASK 来源时 task_point_id 非空且唯一；VOLUNTARY 时为空。
type Observation struct {
	ID              int64      `gorm:"primaryKey;column:id" json:"id"`
	Source          string     `gorm:"column:source" json:"source"`
	TaskPointID     *int64     `gorm:"column:task_point_id;uniqueIndex" json:"task_point_id"`
	ReporterID      int64      `gorm:"column:reporter_id" json:"reporter_id"`
	SegmentID       *int64     `gorm:"column:segment_id" json:"segment_id"`
	PointID         *int64     `gorm:"column:point_id" json:"point_id"`
	LocationText    string     `gorm:"column:location_text" json:"location_text"`
	Latitude        *float64   `gorm:"column:latitude" json:"latitude"`
	Longitude       *float64   `gorm:"column:longitude" json:"longitude"`
	AccuracyM       *float64   `gorm:"column:accuracy_m" json:"accuracy_m"`
	DistanceM       *float64   `gorm:"column:distance_m" json:"distance_m"`
	CapturedAt      *time.Time `gorm:"column:captured_at" json:"captured_at"`
	InspectionOutcome string   `gorm:"column:inspection_outcome" json:"inspection_outcome"`
	Status          string     `gorm:"column:status" json:"status"`
	Revision        int64      `gorm:"column:revision" json:"revision"`
	SubmittedAt     *time.Time `gorm:"column:submitted_at" json:"submitted_at"`
	ReviewedAt      *time.Time `gorm:"column:reviewed_at" json:"reviewed_at"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// RecognitionAttempt 识别尝试；每次重试新建记录，不覆盖历史。
type RecognitionAttempt struct {
	ID           int64      `gorm:"primaryKey;column:id" json:"id"`
	ImageID      int64      `gorm:"column:image_id;index:idx_img_attempt_no" json:"image_id"`
	AttemptNo    int        `gorm:"column:attempt_no;index:idx_img_attempt_no" json:"attempt_no"`
	Status       string     `gorm:"column:status" json:"status"` // PENDING / SUCCEEDED / FAILED
	ModelVersion string     `gorm:"column:model_version" json:"model_version"`
	SchemaVersion string    `gorm:"column:schema_version" json:"schema_version"`
	LatencyMs    int        `gorm:"column:latency_ms" json:"latency_ms"`
	ErrorCode    string     `gorm:"column:error_code" json:"error_code"`
	StartedAt    time.Time  `gorm:"column:started_at" json:"started_at"`
	FinishedAt   *time.Time `gorm:"column:finished_at" json:"finished_at"`
}

// TableName 显式表名。
func (Image) TableName() string              { return "images" }
func (Observation) TableName() string        { return "observations" }
func (RecognitionAttempt) TableName() string { return "recognition_attempts" }
