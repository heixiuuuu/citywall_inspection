package observation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"sync"
	"time"

	"citywall/server/internal/infrastructure/recognizer"
	"citywall/server/internal/middleware"
	"citywall/server/internal/module/audit"
	"citywall/server/internal/module/task"
)

// 领域错误。
var (
	// ErrNotFound 记录/图片不存在（404）。
	ErrNotFound = errors.New("observation: not found")
	// ErrForbidden 无权限（403）。
	ErrForbidden = errors.New("observation: forbidden")
	// ErrInvalidState 状态不允许该操作（409）。
	ErrInvalidState = errors.New("observation: invalid state for operation")
	// ErrStatusConflict revision 冲突（409）。
	ErrStatusConflict = errors.New("observation: status or revision conflict")
	// ErrSourceConstraint 来源约束不满足（400）。
	ErrSourceConstraint = errors.New("observation: source constraint violated")
	// ErrDuplicateRecord 同一任务点已有逻辑记录。
	ErrDuplicateRecord = errors.New("observation: task point already has a record")
	// ErrMissingOverview 缺少现场概览照片。
	ErrMissingOverview = errors.New("observation: at least one overview image required")
	// ErrMissingConclusion 缺少人工结论。
	ErrMissingConclusion = errors.New("observation: human conclusion required before submit")
	// ErrMissingLocation 缺少位置信息。
	ErrMissingLocation = errors.New("observation: location required")
	// ErrSelfReview 不能审核本人提交的记录。
	ErrSelfReview = errors.New("observation: cannot review own record")
	// ErrReturnReasonRequired 退回/驳回必须填写理由。
	ErrReturnReasonRequired = errors.New("observation: return reason required")
	// ErrRejectOnlyVoluntary 驳回仅限主动上报。
	ErrRejectOnlyVoluntary = errors.New("observation: reject only allowed for voluntary")
	// ErrIdempotentReplay 同幂等键重放，不产生副作用。
	ErrIdempotentReplay = errors.New("observation: idempotent replay, no side effects")
)

// Storage 抽象图片落盘（由 infrastructure/storage 实现）。
type Storage interface {
	SaveImage(data []byte, mediaType string) (string, error)
	Open(relKey string) (*os.File, error)
}

// RecognizerClient 抽象识别客户端。
type RecognizerClient interface {
	Recognize(ctx context.Context, attemptID, imageID, requestID string, image []byte) (*recognizer.Result, error)
}

// TaskPointRepository 由 task 模块仓储实现。
type TaskPointRepository interface {
	GetTaskPoint(ctx context.Context, id int64) (*task.TaskPoint, error)
}

// TaskLifecycle 由 task 模块仓储实现：任务首次采集时 ASSIGNED → IN_PROGRESS。
type TaskLifecycle interface {
	MarkInProgressIfAssigned(ctx context.Context, taskID int64) error
}

// TaskPointLister 按任务列出任务点（由 task 模块仓储实现）。
type TaskPointLister interface {
	ListPoints(ctx context.Context, taskID int64) ([]task.TaskPoint, error)
}

// PatchInput PATCH /observations/{id} 允许的修改字段。
type PatchInput struct {
	LocationText      string     `json:"location_text"`
	Latitude          *float64   `json:"latitude"`
	Longitude         *float64   `json:"longitude"`
	AccuracyM         *float64   `json:"accuracy_m"`
	CapturedAt        *time.Time `json:"captured_at"`
	InspectionOutcome string     `json:"inspection_outcome"`
}

// ConclusionItemIn 人工结论项。
type ConclusionItemIn struct {
	ImageID           int64  `json:"image_id" binding:"required"`
	SourceDetectionID *int64 `json:"source_detection_id"`
	Decision          string `json:"decision" binding:"required,oneof=CONFIRM MODIFY REJECT ADD"`
	FinalClassCode    string `json:"final_class_code"`
	FinalBoxJSON      string `json:"final_box_json"`
	Description       string `json:"description"`
}

// ConclusionInput PUT /observations/{id}/conclusion 入参。
type ConclusionInput struct {
	Outcome      string             `json:"outcome" binding:"required,oneof=NORMAL SUSPECTED UNDETERMINED UNREACHABLE"`
	Description  string             `json:"description"`
	PriorityCode string             `json:"priority_code"`
	Items        []ConclusionItemIn `json:"items" binding:"dive"`
}

// ReviewInput POST /observations/{id}/review 入参。
type ReviewInput struct {
	Decision    string `json:"decision" binding:"required,oneof=ACCEPT RETURN REJECT"`
	Reason      string `json:"reason"`
	Disposition string `json:"disposition"`
}

// Service 现场记录业务规则。
type Service interface {
	RemoveImage(ctx context.Context, actor *middleware.Actor, observationID, imageID int64) error
	Evidence(ctx context.Context, actor *middleware.Actor, id int64) (*Evidence, error)
	Create(ctx context.Context, actor *middleware.Actor, source string, taskPointID *int64, locationText string) (*Observation, error)
	Update(ctx context.Context, actor *middleware.Actor, id, revision int64, in PatchInput) (*Observation, error)
	AddImage(ctx context.Context, actor *middleware.Actor, obsID int64, mediaType string, data []byte, role string, idempotencyKey string) (*Image, error)
	GetImageContent(ctx context.Context, actor *middleware.Actor, imageID int64) (*Image, io.ReadCloser, error)
	Recognize(ctx context.Context, actor *middleware.Actor, imageID int64, requestID string) (*RecognitionAttempt, []Detection, error)
	SaveConclusion(ctx context.Context, actor *middleware.Actor, obsID, revision int64, in ConclusionInput) (*ConclusionRevision, error)
	Submit(ctx context.Context, actor *middleware.Actor, obsID, revision int64) (*Observation, error)
	Review(ctx context.Context, actor *middleware.Actor, obsID, revision int64, in ReviewInput) (*Observation, error)
	ListPendingReview(ctx context.Context, actor *middleware.Actor) ([]Observation, error)
	ListImages(ctx context.Context, actor *middleware.Actor, obsID int64) ([]ImageMeta, error)
	List(ctx context.Context, actor *middleware.Actor, in ListInput) (*ListResult, error)

	// TaskPointStatuses 实现 task.ObservationStatusProvider。
	TaskPointStatuses(ctx context.Context, taskID int64) (map[int64]string, error)

	// AcceptedAdmissionInfo 返回已通过审核记录的受理信息（供 issue 模块受理时使用）。
	AcceptedAdmissionInfo(ctx context.Context, obsID int64) (*AcceptedInfo, error)
}

func (s *service) removeImage(ctx context.Context, actor *middleware.Actor, observationID, imageID int64) error {
	o, err := s.repo.GetByID(ctx, observationID)
	if err != nil {
		return ErrNotFound
	}
	if o.ReporterID != actor.UserID && !isAdminOrReviewer(actor) {
		return ErrForbidden
	}
	if o.Status != ObsDraft && o.Status != ObsReturned {
		return ErrInvalidState
	}
	return s.repo.UnlinkImage(ctx, observationID, imageID)
}

type service struct {
	repo            Repository
	storage         Storage
	recognizer      RecognizerClient
	taskPoints      TaskPointRepository
	taskLife        TaskLifecycle
	taskPointLister TaskPointLister
	audit           audit.Recorder
	idemMu          sync.Mutex
	idemResults     map[string]idemResult
	idemStore       IdempotencyStore
}

// IdempotencyStore 落库幂等记录（由 idempotency 模块实现；nil 时回退内存）。
type IdempotencyStore interface {
	// CheckIdempotent 同键已有成功记录返回 true（不重复执行）。
	CheckIdempotent(ctx context.Context, actorID int64, operation, key string) (bool, error)
	// RecordIdempotent 执行成功后补写记录（并发冲突忽略）。
	RecordIdempotent(ctx context.Context, actorID int64, operation, key, reqHash string) error
}

// SetIdempotencyStore 注入落库幂等存储（bootstrap 装配；测试可跳过）。
func (s *service) SetIdempotencyStore(st IdempotencyStore) {
	s.idemStore = st
}

// idemResult 幂等键对应的首次执行结果（图片或错误）。
type idemResult struct {
	img *Image
	err error
}

// NewService 构造现场记录服务。
// rec 为审计记录器（可为 no-op，审计失败不阻断业务）。
func NewService(repo Repository, st Storage, rc RecognizerClient, tp TaskPointRepository, tl TaskLifecycle, lister TaskPointLister, rec audit.Recorder) Service {
	return &service{repo: repo, storage: st, recognizer: rc, taskPoints: tp, taskLife: tl, taskPointLister: lister, audit: rec,
		idemResults: make(map[string]idemResult)}
}

func (s *service) Create(ctx context.Context, actor *middleware.Actor, source string, taskPointID *int64, locationText string) (*Observation, error) {
	o := &Observation{
		Source:       source,
		TaskPointID:  taskPointID,
		ReporterID:   actor.UserID,
		LocationText: locationText,
		Status:       ObsDraft,
		Revision:     1,
	}
	switch source {
	case SourceTask:
		if taskPointID == nil {
			return nil, ErrSourceConstraint
		}
		point, err := s.taskPoints.GetTaskPoint(ctx, *taskPointID)
		if err != nil {
			return nil, ErrNotFound
		}
		if point.AssigneeID != actor.UserID && !isAdminOrReviewer(actor) {
			return nil, ErrForbidden
		}
		if point.TaskStatus != task.StatusAssigned && point.TaskStatus != task.StatusInProgress && point.TaskStatus != task.StatusReturned {
			return nil, ErrInvalidState
		}
		o.PointID = &point.PointID
		o.SegmentID = point.SnapshotSegmentID
		if existing, err := s.repo.GetByTaskPoint(ctx, *taskPointID); err == nil && existing != nil {
			return nil, ErrDuplicateRecord
		}
	case SourceVoluntary:
		if taskPointID != nil {
			return nil, ErrSourceConstraint
		}
		if locationText == "" {
			return nil, ErrMissingLocation
		}
	default:
		return nil, ErrSourceConstraint
	}
	if err := s.repo.Insert(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *service) update(ctx context.Context, actor *middleware.Actor, id, revision int64, in PatchInput) (*Observation, error) {
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if o.ReporterID != actor.UserID && !isAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	if o.Status != ObsDraft && o.Status != ObsReturned {
		return nil, ErrInvalidState
	}
	if o.Revision != revision {
		return nil, ErrStatusConflict
	}
	if in.LocationText != "" {
		o.LocationText = in.LocationText
	}
	if in.Latitude != nil {
		o.Latitude = in.Latitude
	}
	if in.Longitude != nil {
		o.Longitude = in.Longitude
	}
	if in.AccuracyM != nil {
		o.AccuracyM = in.AccuracyM
	}
	if in.CapturedAt != nil {
		o.CapturedAt = in.CapturedAt
	}
	if in.InspectionOutcome != "" {
		o.InspectionOutcome = in.InspectionOutcome
	}
	if err := s.repo.Update(ctx, o); err != nil {
		return nil, err
	}
	if count, err := s.repo.BumpRevision(ctx, id, revision); err != nil {
		return nil, err
	} else if count == 0 {
		return nil, ErrStatusConflict
	}
	o.Revision++
	return o, nil
}

func (s *service) AddImage(ctx context.Context, actor *middleware.Actor, obsID int64, mediaType string, data []byte, role string, idempotencyKey string) (*Image, error) {
	// 幂等键：同键已成功执行过 → 直接返回重放标记，不重复落图。
	// 首次执行：先查（不写记录），成功后补写；执行失败不写，允许同键重试。
	if idempotencyKey != "" {
		if s.idemStore != nil {
			done, err := s.idemStore.CheckIdempotent(ctx, actor.UserID, "upload_image", idempotencyKey)
			if err == nil && done {
				return nil, ErrIdempotentReplay
			}
		} else {
			s.idemMu.Lock()
			if cached, ok := s.idemResults[idempotencyKey]; ok {
				s.idemMu.Unlock()
				return cached.img, cached.err
			}
			s.idemMu.Unlock()
		}
	}
	o, err := s.repo.GetByID(ctx, obsID)
	if err != nil {
		return nil, ErrNotFound
	}
	if o.ReporterID != actor.UserID && !isAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	if role != ImageRoleOverview && role != ImageRoleDetail {
		return nil, ErrSourceConstraint
	}
	if o.Status != ObsDraft && o.Status != ObsReturned {
		return nil, ErrInvalidState
	}

	key, err := s.storage.SaveImage(data, mediaType)
	if err != nil {
		return nil, err
	}
	w, h := imageSize(data)
	img := &Image{
		StorageKey: key,
		SHA256:     sha256Hex(data),
		MediaType:  mediaType,
		ByteSize:   int64(len(data)),
		Width:      w,
		Height:     h,
		UploadedBy: actor.UserID,
	}
	if err := s.repo.InsertImage(ctx, img); err != nil {
		return nil, err
	}
	if err := s.repo.LinkImage(ctx, obsID, img.ID, role); err != nil {
		return nil, err
	}
	if idempotencyKey != "" {
		if s.idemStore != nil {
			_ = s.idemStore.RecordIdempotent(ctx, actor.UserID, "upload_image", idempotencyKey, sha256Hex(data))
		} else {
			s.idemMu.Lock()
			s.idemResults[idempotencyKey] = idemResult{img: img}
			s.idemMu.Unlock()
		}
	}
	return img, nil
}

func (s *service) GetImageContent(ctx context.Context, actor *middleware.Actor, imageID int64) (*Image, io.ReadCloser, error) {
	img, err := s.repo.GetImage(ctx, imageID)
	if err != nil {
		return nil, nil, ErrNotFound
	}
	allowed, err := s.repo.CanReadImage(ctx, imageID, actor.UserID, isAdminOrReviewer(actor))
	if err != nil {
		return nil, nil, err
	}
	if !allowed {
		return nil, nil, ErrForbidden
	}
	f, err := s.storage.Open(img.StorageKey)
	if err != nil {
		return nil, nil, err
	}
	return img, f, nil
}

func (s *service) Recognize(ctx context.Context, actor *middleware.Actor, imageID int64, requestID string) (*RecognitionAttempt, []Detection, error) {
	img, err := s.repo.GetImage(ctx, imageID)
	if err != nil {
		return nil, nil, ErrNotFound
	}
	o, err := s.repo.ObservationByImage(ctx, imageID)
	if err != nil {
		return nil, nil, ErrNotFound
	}
	if o.ReporterID != actor.UserID && !isAdminOrReviewer(actor) {
		return nil, nil, ErrForbidden
	}
	f, err := s.storage.Open(img.StorageKey)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}

	attempt, err := transactionValue(ctx, s, func(tx *service) (*RecognitionAttempt, error) {
		current, err := tx.repo.GetByID(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		if current.Status != ObsDraft && current.Status != ObsReturned {
			return nil, ErrInvalidState
		}
		attemptNo, err := tx.repo.NextAttemptNo(ctx, imageID)
		if err != nil {
			return nil, err
		}
		attempt := &RecognitionAttempt{ImageID: imageID, AttemptNo: attemptNo, Status: AttemptPending, StartedAt: time.Now().UTC()}
		return attempt, tx.repo.InsertAttempt(ctx, attempt)
	})
	if err != nil {
		return nil, nil, err
	}

	attemptID := fmt.Sprintf("ra_%d", attempt.ID)
	res, err := s.recognizer.Recognize(ctx, attemptID, fmt.Sprintf("img_%d", imageID), requestID, data)
	now := time.Now().UTC()
	attempt.FinishedAt = &now
	if err != nil {
		attempt.Status = AttemptFailed
		attempt.ErrorCode = recognizer.ErrorCode(err)
		if saveErr := s.repo.UpdateAttempt(ctx, attempt); saveErr != nil {
			return nil, nil, saveErr
		}
		return attempt, nil, err
	}
	attempt.Status = AttemptSucceeded
	if len(res.Analysis) > 0 {
		attempt.AnalysisJSON = string(res.Analysis)
	}
	attempt.ModelVersion = res.ModelVersion
	attempt.SchemaVersion = res.SchemaVersion
	attempt.LatencyMs = res.LatencyMs
	detections := make([]Detection, 0, len(res.Detections))
	for _, d := range res.Detections {
		detections = append(detections, Detection{
			AttemptID:    attempt.ID,
			DetectionKey: d.DetectionID,
			ClassCode:    d.ClassCode,
			Confidence:   d.Confidence,
			X1:           d.BBox.X1,
			Y1:           d.BBox.Y1,
			X2:           d.BBox.X2,
			Y2:           d.BBox.Y2,
		})
	}
	if err := s.repo.Transaction(ctx, func(repo Repository) error {
		if err := repo.InsertDetections(ctx, detections); err != nil {
			return err
		}
		return repo.UpdateAttempt(ctx, attempt)
	}); err != nil {
		attempt.Status = AttemptFailed
		attempt.ErrorCode = "RECOGNITION_PERSIST_FAILED"
		if saveErr := s.repo.UpdateAttempt(ctx, attempt); saveErr != nil {
			return nil, nil, saveErr
		}
		return attempt, nil, err
	}
	return attempt, detections, nil
}

func (s *service) saveConclusion(ctx context.Context, actor *middleware.Actor, obsID, revision int64, in ConclusionInput) (*ConclusionRevision, error) {
	o, err := s.repo.GetByID(ctx, obsID)
	if err != nil {
		return nil, ErrNotFound
	}
	if o.ReporterID != actor.UserID && !isAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	if o.Status != ObsDraft && o.Status != ObsReturned {
		return nil, ErrInvalidState
	}
	if o.Revision != revision {
		return nil, ErrStatusConflict
	}

	links, err := s.repo.ListImages(ctx, obsID)
	if err != nil {
		return nil, err
	}
	validImage := map[int64]bool{}
	for _, l := range links {
		validImage[l.ImageID] = true
	}
	items := make([]ConclusionItem, 0, len(in.Items))
	for _, it := range in.Items {
		if !validImage[it.ImageID] {
			return nil, ErrForbidden
		}
		if it.SourceDetectionID != nil {
			belongs, err := s.repo.DetectionBelongsToImage(ctx, *it.SourceDetectionID, it.ImageID)
			if err != nil {
				return nil, err
			}
			if !belongs {
				return nil, ErrForbidden
			}
		}
		items = append(items, ConclusionItem{
			ImageID:           it.ImageID,
			SourceDetectionID: it.SourceDetectionID,
			Decision:          it.Decision,
			FinalClassCode:    it.FinalClassCode,
			FinalBoxJSON:      it.FinalBoxJSON,
			Description:       it.Description,
		})
	}
	no, err := s.repo.NextConclusionNo(ctx, obsID)
	if err != nil {
		return nil, err
	}
	rev := &ConclusionRevision{
		ObservationID: obsID,
		RevisionNo:    no,
		Outcome:       in.Outcome,
		Description:   in.Description,
		PriorityCode:  in.PriorityCode,
		AuthorID:      actor.UserID,
	}
	if err := s.repo.InsertConclusion(ctx, rev, items); err != nil {
		return nil, err
	}
	if count, err := s.repo.BumpRevision(ctx, obsID, revision); err != nil {
		return nil, err
	} else if count == 0 {
		return nil, ErrStatusConflict
	}
	o.Revision++
	return rev, nil
}

func (s *service) submit(ctx context.Context, actor *middleware.Actor, obsID, revision int64) (*Observation, error) {
	o, err := s.repo.GetByID(ctx, obsID)
	if err != nil {
		return nil, ErrNotFound
	}
	if o.ReporterID != actor.UserID && !isAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	if o.Status != ObsDraft && o.Status != ObsReturned {
		return nil, ErrInvalidState
	}
	if o.Revision != revision {
		return nil, ErrStatusConflict
	}

	links, err := s.repo.ListImages(ctx, obsID)
	if err != nil {
		return nil, err
	}
	hasOverview := false
	imageIDs := make([]int64, 0, len(links))
	for _, l := range links {
		imageIDs = append(imageIDs, l.ImageID)
		if l.ImageRole == ImageRoleOverview {
			hasOverview = true
		}
	}
	if !hasOverview {
		return nil, ErrMissingOverview
	}
	conclusion, err := s.repo.LatestConclusion(ctx, obsID)
	if err != nil {
		return nil, ErrMissingConclusion
	}
	items, err := s.repo.ListConclusionItems(ctx, conclusion.ID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		present := false
		for _, imageID := range imageIDs {
			if item.ImageID == imageID {
				present = true
				break
			}
		}
		if !present {
			return nil, fmt.Errorf("%w: human conclusion references a removed photo; save the conclusion again", ErrSourceConstraint)
		}
	}
	if o.Source == SourceVoluntary && (o.LocationText == "" || o.Latitude == nil) {
		return nil, ErrMissingLocation
	}
	if o.Source == SourceTask && o.LocationText == "" {
		return nil, ErrMissingLocation
	}

	locSnap, _ := json.Marshal(map[string]any{
		"location_text": o.LocationText,
		"latitude":      o.Latitude,
		"longitude":     o.Longitude,
		"accuracy_m":    o.AccuracyM,
		"captured_at":   o.CapturedAt,
		"distance_m":    o.DistanceM,
	})
	imgSnap, _ := json.Marshal(imageIDs)

	subNo, err := s.repo.NextSubmissionNo(ctx, obsID)
	if err != nil {
		return nil, err
	}
	sub := &ObservationSubmission{
		ObservationID:        obsID,
		SubmissionNo:         subNo,
		ObservationRevision:  revision,
		ConclusionRevisionID: &conclusion.ID,
		LocationSnapshotJSON: string(locSnap),
		ImageIDsSnapshotJSON: string(imgSnap),
		SubmittedAt:          time.Now().UTC(),
	}
	if err := s.repo.InsertSubmission(ctx, sub); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	n, err := s.repo.UpdateStatus(ctx, obsID, revision, ObsSubmitted)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrStatusConflict
	}
	o.Status = ObsSubmitted
	o.InspectionOutcome = conclusion.Outcome
	o.Revision++
	o.SubmittedAt = &now

	if o.Source == SourceTask && o.TaskPointID != nil {
		if tp, err := s.taskPoints.GetTaskPoint(ctx, *o.TaskPointID); err == nil {
			_ = s.taskLife.MarkInProgressIfAssigned(ctx, tp.TaskID)
		}
	}
	s.audit.Record(ctx, &audit.Event{
		EntityType: "observation",
		EntityID:   o.ID,
		Action:     "submit",
		ActorID:    actor.UserID,
		OldStatus:  ObsDraft,
		NewStatus:  ObsSubmitted,
		Revision:   &o.Revision,
	})
	return o, nil
}

func (s *service) review(ctx context.Context, actor *middleware.Actor, obsID, revision int64, in ReviewInput) (*Observation, error) {
	if !isAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	o, err := s.repo.GetByID(ctx, obsID)
	if err != nil {
		return nil, ErrNotFound
	}
	if o.ReporterID == actor.UserID {
		return nil, ErrSelfReview
	}
	if o.Status != ObsSubmitted {
		return nil, ErrInvalidState
	}
	if o.Revision != revision {
		return nil, ErrStatusConflict
	}
	if in.Decision == ReviewReturn && in.Reason == "" {
		return nil, ErrReturnReasonRequired
	}
	if in.Decision == ReviewReject && o.Source != SourceVoluntary {
		return nil, ErrRejectOnlyVoluntary
	}
	if in.Decision == ReviewReject && in.Reason == "" {
		return nil, ErrReturnReasonRequired
	}

	sub, err := s.repo.LatestSubmission(ctx, obsID)
	if err != nil {
		return nil, ErrNotFound
	}
	decision := &ReviewDecision{
		SubmissionID: sub.ID,
		ReviewerID:   actor.UserID,
		Decision:     in.Decision,
		Disposition:  in.Disposition,
		Reason:       in.Reason,
	}
	if err := s.repo.InsertReviewDecision(ctx, decision); err != nil {
		return nil, err
	}

	var newStatus string
	switch in.Decision {
	case ReviewAccept:
		newStatus = ObsAccepted
	case ReviewReturn:
		newStatus = ObsReturned
	case ReviewReject:
		newStatus = ObsRejected
	}
	now := time.Now().UTC()
	n, err := s.repo.UpdateStatus(ctx, obsID, revision, newStatus)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrStatusConflict
	}
	o.Status = newStatus
	o.Revision++
	o.ReviewedAt = &now
	s.audit.Record(ctx, &audit.Event{
		EntityType: "observation",
		EntityID:   o.ID,
		Action:     "review",
		ActorID:    actor.UserID,
		OldStatus:  ObsSubmitted,
		NewStatus:  newStatus,
		Revision:   &o.Revision,
		Reason:     in.Reason,
	})
	return o, nil
}

func (s *service) ListPendingReview(ctx context.Context, actor *middleware.Actor) ([]Observation, error) {
	if !isAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	return s.repo.ListByStatuses(ctx, []string{ObsSubmitted})
}

// ListInput 记录列表查询条件。
type ListInput struct {
	TaskPointID int64
	Status      string
	TaskID      int64
	Source      string
	PointID     int64
	DateFrom    string
	DateTo      string
	Page        int
	PageSize    int
}

// ListResult 分页列表结果。
type ListResult struct {
	Items []Observation
	Total int64
}

// List 记录列表：管理员/审核员看全部，巡检员只看自己上报；支持多条件过滤 + 分页。
func (s *service) List(ctx context.Context, actor *middleware.Actor, in ListInput) (*ListResult, error) {
	isAdmin := isAdminOrReviewer(actor)
	page, pageSize := in.Page, in.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	limit, offset := pageSize, (page-1)*pageSize
	q := ListQuery{
		TaskPointID: in.TaskPointID,
		ReporterID:  actor.UserID,
		IsAdmin:     isAdmin,
		Status:      in.Status,
		TaskID:      in.TaskID,
		Source:      in.Source,
		PointID:     in.PointID,
		Limit:       limit,
		Offset:      offset,
	}
	if in.DateFrom != "" {
		if t, err := time.Parse(time.RFC3339, in.DateFrom); err == nil {
			q.DateFrom = &t
		}
	}
	if in.DateTo != "" {
		if t, err := time.Parse(time.RFC3339, in.DateTo); err == nil {
			q.DateTo = &t
		}
	}
	items, err := s.repo.ListPage(ctx, q)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountObservations(ctx, q)
	if err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total}, nil
}

// ImageMeta 图片元数据 + 受控访问 URL（不暴露存储路径）。
type ImageMeta struct {
	ID        int64  `json:"id"`
	Role      string `json:"role"`
	MediaType string `json:"media_type"`
	ByteSize  int64  `json:"byte_size"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	URL       string `json:"url"`
}

func imageMetadata(image *Image, role string) ImageMeta {
	return ImageMeta{ID: image.ID, Role: role, MediaType: image.MediaType, ByteSize: image.ByteSize,
		Width: image.Width, Height: image.Height, URL: fmt.Sprintf("/api/v1/images/%d/content", image.ID)}
}

// ListImages 按现场记录读取权限返回关联图片元数据。
func (s *service) ListImages(ctx context.Context, actor *middleware.Actor, obsID int64) ([]ImageMeta, error) {
	_, err := s.repo.GetByID(ctx, obsID)
	if err != nil {
		return nil, ErrNotFound
	}
	allowed, err := s.repo.CanReadObservation(ctx, obsID, actor.UserID, isAdminOrReviewer(actor))
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	links, err := s.repo.ListImages(ctx, obsID)
	if err != nil {
		return nil, err
	}
	metas := make([]ImageMeta, 0, len(links))
	for _, l := range links {
		img, err := s.repo.GetImage(ctx, l.ImageID)
		if err != nil {
			continue // 图片记录缺失时跳过，不阻断列表
		}
		metas = append(metas, imageMetadata(img, l.ImageRole))
	}
	return metas, nil
}

// TaskPointStatuses 实现 task.ObservationStatusProvider。
func (s *service) TaskPointStatuses(ctx context.Context, taskID int64) (map[int64]string, error) {
	if s.taskPointLister == nil {
		return nil, errors.New("observation: task point repository is unavailable")
	}
	points, err := s.taskPointLister.ListPoints(ctx, taskID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(points))
	for _, p := range points {
		ids = append(ids, p.ID)
	}
	return s.repo.StatusByTaskPoints(ctx, ids)
}

// AcceptedAdmissionInfo 仅 ACCEPTED 记录可受理；返回最近审核决定 ID 与位置快照。
func (s *service) AcceptedAdmissionInfo(ctx context.Context, obsID int64) (*AcceptedInfo, error) {
	o, err := s.repo.GetByID(ctx, obsID)
	if err != nil {
		return nil, ErrNotFound
	}
	if o.Status != ObsAccepted {
		return nil, ErrInvalidState
	}
	dec, err := s.repo.LatestReviewDecision(ctx, obsID)
	if err != nil {
		return nil, err
	}
	return &AcceptedInfo{
		Source:           o.Source,
		SegmentID:        o.SegmentID,
		PointID:          o.PointID,
		LocationText:     o.LocationText,
		ReviewDecisionID: dec.ID,
	}, nil
}

func isAdminOrReviewer(actor *middleware.Actor) bool {
	if actor == nil {
		return false
	}
	for _, r := range actor.Roles {
		if r == "ADMIN" || r == "REVIEWER" {
			return true
		}
	}
	return false
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func imageSize(data []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}
