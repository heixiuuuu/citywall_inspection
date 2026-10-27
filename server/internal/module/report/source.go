package report

import (
	"context"
	"time"
)

// ---------- 快照视图结构（同时用于 snapshot_json 与 PDF 渲染） ----------

// ImageView 图片元数据（报告嵌入与引用）。
type ImageView struct {
	ID         int64  `json:"id"`
	StorageKey string `json:"storage_key"`
	MediaType  string `json:"media_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Role       string `json:"role"` // OVERVIEW / DETAIL
}

// DetectionView 模型候选检测框（原图像素坐标）。
type DetectionView struct {
	ClassCode  string  `json:"class_code"`
	Confidence float64 `json:"confidence"`
	X1         int     `json:"x1"`
	Y1         int     `json:"y1"`
	X2         int     `json:"x2"`
	Y2         int     `json:"y2"`
}

// AttemptView 识别尝试（多次识别分别留存）。
type AttemptView struct {
	AnalysisJSON string          `json:"analysis_json"`
	AttemptNo    int             `json:"attempt_no"`
	Status       string          `json:"status"`
	ModelVersion string          `json:"model_version"`
	ErrorCode    string          `json:"error_code"`
	LatencyMs    int             `json:"latency_ms"`
	Detections   []DetectionView `json:"detections"`
}

// ItemView 人工结论项（确认/修改/误检/补录）。
type ItemView struct {
	ImageID           int64  `json:"image_id"`
	Decision          string `json:"decision"`
	FinalClassCode    string `json:"final_class_code"`
	FinalBoxJSON      string `json:"final_box_json"`
	Description       string `json:"description"`
	SourceDetectionID *int64 `json:"source_detection_id"`
}

// ConclusionView 人工结论版本（revision_no 历史序）。
type ConclusionView struct {
	RevisionNo   int        `json:"revision_no"`
	Outcome      string     `json:"outcome"`
	Description  string     `json:"description"`
	PriorityCode string     `json:"priority_code"`
	AuthorID     int64      `json:"author_id"`
	CreatedAt    time.Time  `json:"created_at"`
	Items        []ItemView `json:"items"`
}

// ReviewView 审核决定。
type ReviewView struct {
	Decision    string    `json:"decision"`
	ReviewerID  int64     `json:"reviewer_id"`
	Disposition string    `json:"disposition"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

// ObservationView 现场记录完整视图。
type ObservationView struct {
	AdmittedAt       *time.Time       `json:"admitted_at"`
	AdmittedBy       *int64           `json:"admitted_by"`
	NoIssueReason    string           `json:"no_issue_reason"`
	ID               int64            `json:"id"`
	Source           string           `json:"source"`
	TaskPointID      *int64           `json:"task_point_id"`
	ReporterID       int64            `json:"reporter_id"`
	LocationText     string           `json:"location_text"`
	Latitude         *float64         `json:"latitude"`
	Longitude        *float64         `json:"longitude"`
	AccuracyM        *float64         `json:"accuracy_m"`
	DistanceM        *float64         `json:"distance_m"`
	Status           string           `json:"status"`
	Outcome          string           `json:"outcome"`
	SubmittedAt      *time.Time       `json:"submitted_at"`
	ReviewedAt       *time.Time       `json:"reviewed_at"`
	Images           []ImageView      `json:"images"`
	Attempts         []AttemptView    `json:"attempts"`
	Conclusions      []ConclusionView `json:"conclusions"`
	Review           *ReviewView      `json:"review"`
	LocationAbnormal string           `json:"location_abnormal"` // 定位异常说明（distance/accuracy 超限）
}

type IssueActionView struct {
	Images            []ImageView `json:"images" gorm:"-"`
	ID                int64       `json:"id"`
	IssueID           int64       `json:"issue_id"`
	ActorID           int64       `json:"actor_id"`
	ActorName         string      `json:"actor_name"`
	ActionType        string      `json:"action_type"`
	Content           string      `json:"content"`
	PlannedFollowUpAt *time.Time  `json:"planned_follow_up_at"`
	CreatedAt         time.Time   `json:"created_at"`
}

// IssueLinkView 关联问题摘要。
type IssueLinkView struct {
	AssigneeName string            `json:"assignee_name"`
	Actions      []IssueActionView `json:"actions" gorm:"-"`
	ID           int64             `json:"id"`
	Title        string            `json:"title"`
	Status       string            `json:"status"`
	PriorityCode string            `json:"priority_code"`
	AssigneeID   *int64            `json:"assignee_id"`
	OpenedAt     time.Time         `json:"opened_at"`
	ClosedAt     *time.Time        `json:"closed_at"`
	CloseReason  string            `json:"close_reason"`
}

// PointView 任务点视图（含覆盖状态与关联记录）。
type PointView struct {
	TaskPointID      int64            `json:"task_point_id"`
	SequenceNo       int              `json:"sequence_no"`
	PointName        string           `json:"point_name"`
	PointCode        string           `json:"point_code"`
	SnapshotLocation string           `json:"snapshot_location"`
	Observation      *ObservationView `json:"observation"` // 有记录时非空
}

// TaskSource 任务报告快照。
type TaskSource struct {
	TaskID       int64           `json:"task_id"`
	Title        string          `json:"title"`
	AssigneeID   int64           `json:"assignee_id"`
	AssigneeName string          `json:"assignee_name"`
	DueAt        *time.Time      `json:"due_at"`
	Status       string          `json:"status"`
	Description  string          `json:"description"`
	DispatchedAt *time.Time      `json:"dispatched_at"`
	CompletedAt  *time.Time      `json:"completed_at"`
	Points       []PointView     `json:"points"`
	Issues       []IssueLinkView `json:"issues"`
	// 汇总统计。
	PointTotal       int `json:"point_total"`
	PointCovered     int `json:"point_covered"`     // 有有效记录（SUBMITTED 及以上）
	PointNormal      int `json:"point_normal"`      // 结论 NORMAL
	PointSuspected   int `json:"point_suspected"`   // 结论 SUSPECTED
	PointUnreachable int `json:"point_unreachable"` // 结论 UNREACHABLE 且审核批准
	PointUncovered   int `json:"point_uncovered"`   // 本次未覆盖（含无法到达批准）
	RejectedCount    int `json:"rejected_count"`    // 模型候选被人工判定误检数
	MissingCount     int `json:"missing_count"`     // 人工补录（模型漏检）数
	RetryCount       int `json:"retry_count"`       // 重新识别次数
}

// RangeSource 时间范围报告快照。
type RangeSource struct {
	Start            time.Time         `json:"start"`
	End              time.Time         `json:"end"`
	Observations     []ObservationView `json:"observations"`
	Issues           []IssueLinkView   `json:"issues"`
	TotalRecords     int               `json:"total_records"`
	SubmittedCount   int               `json:"submitted_count"`
	AcceptedCount    int               `json:"accepted_count"`
	ReturnedCount    int               `json:"returned_count"`
	RejectedCount    int               `json:"rejected_count"`
	SuspectedCount   int               `json:"suspected_count"`
	NormalCount      int               `json:"normal_count"`
	UnreachableCount int               `json:"unreachable_count"`
}

// SnapshotSource 由本模块仓储实现：读取各业务表生成只读快照。
type SnapshotSource interface {
	// LoadTask 加载任务报告所需的全部数据。
	LoadTask(ctx context.Context, taskID int64) (*TaskSource, error)
	// LoadRange 加载时间范围报告所需的全部数据。
	LoadRange(ctx context.Context, start, end time.Time) (*RangeSource, error)
}
