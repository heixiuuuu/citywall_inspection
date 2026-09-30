// Package observation 承载现场记录、图片、识别尝试、人工结论与审核决定领域模型。
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
	OutcomeNormal       = "NORMAL"
	OutcomeSuspected    = "SUSPECTED"
	OutcomeUndetermined = "UNDETERMINED"
	OutcomeUnreachable  = "UNREACHABLE"
)

// 记录状态（04 文档 7.2）。
const (
	ObsDraft    = "DRAFT"
	ObsSubmitted = "SUBMITTED"
	ObsReturned = "RETURNED"
	ObsAccepted = "ACCEPTED"
	ObsRejected = "REJECTED"
)

// 图片角色。
const (
	ImageRoleOverview = "OVERVIEW" // 现场概览图（任务点必传）
	ImageRoleDetail   = "DETAIL"   // 异常细节图
)

// 识别尝试状态。
const (
	AttemptPending   = "PENDING"
	AttemptSucceeded = "SUCCEEDED"
	AttemptFailed    = "FAILED"
)

// 人工结论项决策。
const (
	DecisionConfirm = "CONFIRM"
	DecisionModify  = "MODIFY"
	DecisionReject  = "REJECT"
	DecisionAdd     = "ADD"
)

// 审核决定。
const (
	ReviewAccept = "ACCEPT"
	ReviewReturn = "RETURN"
	ReviewReject = "REJECT" // 仅主动上报驳回
)

// Image 图片元数据；storage_key 由服务端生成，目录不公开静态访问。
type Image struct {
	ID         int64      `gorm:"primaryKey;column:id" json:"id"`
	StorageKey string     `gorm:"column:storage_key;uniqueIndex" json:"storage_key"`
	SHA256     string     `gorm:"column:sha256" json:"sha256"`
	MediaType  string     `gorm:"column:media_type" json:"media_type"`
	ByteSize   int64      `gorm:"column:byte_size" json:"byte_size"`
	Width      int        `gorm:"column:width" json:"width"`
	Height     int        `gorm:"column:height" json:"height"`
	CapturedAt *time.Time `gorm:"column:captured_at" json:"captured_at"`
	UploadedBy int64      `gorm:"column:uploaded_by" json:"uploaded_by"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
}

// Observation 现场记录。TASK 来源时 task_point_id 非空且唯一；VOLUNTARY 时为空。
type Observation struct {
	ID                int64      `gorm:"primaryKey;column:id" json:"id"`
	Source            string     `gorm:"column:source" json:"source"`
	TaskPointID       *int64     `gorm:"column:task_point_id;uniqueIndex" json:"task_point_id"`
	ReporterID        int64      `gorm:"column:reporter_id" json:"reporter_id"`
	SegmentID         *int64     `gorm:"column:segment_id" json:"segment_id"`
	PointID           *int64     `gorm:"column:point_id" json:"point_id"`
	LocationText      string     `gorm:"column:location_text" json:"location_text"`
	Latitude          *float64   `gorm:"column:latitude" json:"latitude"`
	Longitude         *float64   `gorm:"column:longitude" json:"longitude"`
	AccuracyM         *float64   `gorm:"column:accuracy_m" json:"accuracy_m"`
	DistanceM         *float64   `gorm:"column:distance_m" json:"distance_m"`
	CapturedAt        *time.Time `gorm:"column:captured_at" json:"captured_at"`
	InspectionOutcome string     `gorm:"column:inspection_outcome" json:"inspection_outcome"`
	Status            string     `gorm:"column:status" json:"status"`
	Revision          int64      `gorm:"column:revision" json:"revision"`
	SubmittedAt       *time.Time `gorm:"column:submitted_at" json:"submitted_at"`
	ReviewedAt        *time.Time `gorm:"column:reviewed_at" json:"reviewed_at"`
	CreatedAt         time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// ObservationImage 记录与图片的关系及角色。
type ObservationImage struct {
	ObservationID int64  `gorm:"primaryKey;column:observation_id" json:"observation_id"`
	ImageID       int64  `gorm:"primaryKey;column:image_id" json:"image_id"`
	ImageRole     string `gorm:"column:image_role" json:"image_role"`
}

// ObservationSubmission 每次提交冻结的位置、图片 ID 与人工结论版本。
type ObservationSubmission struct {
	ID                   int64     `gorm:"primaryKey;column:id" json:"id"`
	ObservationID        int64     `gorm:"column:observation_id;index:idx_obs_submission_no" json:"observation_id"`
	SubmissionNo         int       `gorm:"column:submission_no;index:idx_obs_submission_no" json:"submission_no"`
	ObservationRevision  int64     `gorm:"column:observation_revision" json:"observation_revision"`
	ConclusionRevisionID *int64    `gorm:"column:conclusion_revision_id" json:"conclusion_revision_id"`
	LocationSnapshotJSON string    `gorm:"column:location_snapshot_json;type:text" json:"location_snapshot_json"`
	ImageIDsSnapshotJSON string    `gorm:"column:image_ids_snapshot_json;type:text" json:"image_ids_snapshot_json"`
	SubmittedAt          time.Time `gorm:"column:submitted_at" json:"submitted_at"`
}

// ReviewDecision 一个提交版本至多一个最终审核决定。
type ReviewDecision struct {
	ID          int64     `gorm:"primaryKey;column:id" json:"id"`
	SubmissionID int64    `gorm:"column:submission_id;uniqueIndex" json:"submission_id"`
	ReviewerID  int64     `gorm:"column:reviewer_id" json:"reviewer_id"`
	Decision    string    `gorm:"column:decision" json:"decision"` // ACCEPT / RETURN / REJECT
	Disposition string    `gorm:"column:disposition" json:"disposition"`
	Reason      string    `gorm:"column:reason" json:"reason"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

// RecognitionAttempt 识别尝试；每次重试新建记录，不覆盖历史。
type RecognitionAttempt struct {
	ID            int64      `gorm:"primaryKey;column:id" json:"id"`
	ImageID       int64      `gorm:"column:image_id;index:idx_img_attempt_no" json:"image_id"`
	AttemptNo     int        `gorm:"column:attempt_no;index:idx_img_attempt_no" json:"attempt_no"`
	Status        string     `gorm:"column:status" json:"status"` // PENDING / SUCCEEDED / FAILED
	ModelVersion  string     `gorm:"column:model_version" json:"model_version"`
	SchemaVersion string     `gorm:"column:schema_version" json:"schema_version"`
	LatencyMs     int        `gorm:"column:latency_ms" json:"latency_ms"`
	ErrorCode     string     `gorm:"column:error_code" json:"error_code"`
	StartedAt     time.Time  `gorm:"column:started_at" json:"started_at"`
	FinishedAt    *time.Time `gorm:"column:finished_at" json:"finished_at"`
}

// Detection 候选检测框（原图像素坐标）。
type Detection struct {
	ID           int64   `gorm:"primaryKey;column:id" json:"id"`
	AttemptID    int64   `gorm:"column:attempt_id;index:idx_attempt_detection_key" json:"attempt_id"`
	DetectionKey string  `gorm:"column:detection_key;index:idx_attempt_detection_key" json:"detection_key"`
	ClassCode    string  `gorm:"column:class_code" json:"class_code"`
	Confidence   float64 `gorm:"column:confidence" json:"confidence"`
	X1           int     `gorm:"column:x1" json:"x1"`
	Y1           int     `gorm:"column:y1" json:"y1"`
	X2           int     `gorm:"column:x2" json:"x2"`
	Y2           int     `gorm:"column:y2" json:"y2"`
}

// ConclusionRevision 每次人工结论版本（revision_no 为不可变历史序号）。
type ConclusionRevision struct {
	ID            int64     `gorm:"primaryKey;column:id" json:"id"`
	ObservationID int64     `gorm:"column:observation_id;index:idx_obs_conclusion_no" json:"observation_id"`
	RevisionNo    int       `gorm:"column:revision_no;index:idx_obs_conclusion_no" json:"revision_no"`
	Outcome       string    `gorm:"column:outcome" json:"outcome"`
	Description   string    `gorm:"column:description" json:"description"`
	PriorityCode  string    `gorm:"column:priority_code" json:"priority_code"`
	AuthorID      int64     `gorm:"column:author_id" json:"author_id"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

// ConclusionItem 人工结论项：确认/修改/删除候选或人工补录。
type ConclusionItem struct {
	ID                  int64  `gorm:"primaryKey;column:id" json:"id"`
	ConclusionRevisionID int64 `gorm:"column:conclusion_revision_id;index" json:"conclusion_revision_id"`
	ImageID             int64  `gorm:"column:image_id" json:"image_id"`
	SourceDetectionID   *int64 `gorm:"column:source_detection_id" json:"source_detection_id"`
	Decision            string `gorm:"column:decision" json:"decision"`
	FinalClassCode      string `gorm:"column:final_class_code" json:"final_class_code"`
	FinalBoxJSON        string `gorm:"column:final_box_json" json:"final_box_json"`
	Description         string `gorm:"column:description" json:"description"`
}

// TableName 显式表名。
func (Image) TableName() string                { return "images" }
func (Observation) TableName() string          { return "observations" }
func (ObservationImage) TableName() string     { return "observation_images" }
func (ObservationSubmission) TableName() string { return "observation_submissions" }
func (ReviewDecision) TableName() string       { return "review_decisions" }
func (RecognitionAttempt) TableName() string   { return "recognition_attempts" }
func (Detection) TableName() string            { return "detections" }
func (ConclusionRevision) TableName() string   { return "conclusion_revisions" }
func (ConclusionItem) TableName() string       { return "conclusion_items" }
