package task

import (
	"context"
	"errors"
	"time"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/audit"
)

// 领域错误（Handler 映射 HTTP 状态）。
var (
	// ErrNotImplemented 占位错误。
	ErrNotImplemented = errors.New("task: not implemented yet")
	// ErrStatusConflict 状态或 revision 冲突（409）。
	ErrStatusConflict = errors.New("task: status or revision conflict")
	// ErrNotFound 任务不存在（404）。
	ErrNotFound = errors.New("task: not found")
	// ErrForbidden 无权限（403）。
	ErrForbidden = errors.New("task: forbidden")
	// ErrInvalidState 当前状态不允许该操作（409）。
	ErrInvalidState = errors.New("task: invalid state for operation")
	// ErrDuplicatePoint 点位重复。
	ErrDuplicatePoint = errors.New("task: duplicate inspection point")
	// ErrNoPoints 任务至少需要一个点位。
	ErrNoPoints = errors.New("task: at least one inspection point required")
	// ErrPointNotFound 点位不存在。
	ErrPointNotFound = errors.New("task: inspection point not found")
	// ErrUnfinishedPoints 存在未完成的任务点。
	ErrUnfinishedPoints = errors.New("task: some inspection points not completed")
)

// ObservationStatusProvider 由 observation 模块实现，避免 task 反向 import observation。
// 用于任务提交/完成时核对各任务点的记录审核状态。
type ObservationStatusProvider interface {
	// TaskPointStatuses 返回任务各 task_point 的记录状态（key 为 task_point_id；无记录则缺省）。
	TaskPointStatuses(ctx context.Context, taskID int64) (map[int64]string, error)
}

// CreateInput 创建任务入参。
type CreateInput struct {
	Title       string     `json:"title" binding:"required"`
	AssigneeID  int64      `json:"assignee_id" binding:"required"`
	DueAt       *time.Time `json:"due_at"`
	Description string     `json:"description"`
	PointIDs    []int64    `json:"point_ids" binding:"required,min=1"`
}

// Repository 任务数据访问接口。
type Repository interface {
	InsertTask(ctx context.Context, t *Task) error
	InsertTaskPoints(ctx context.Context, taskID int64, pointIDs []int64) error
	GetByID(ctx context.Context, id int64) (*Task, error)
	ListByActor(ctx context.Context, actorID int64, isAdmin bool) ([]Task, error)
	ListPage(ctx context.Context, actorID int64, isAdmin bool, status string, limit, offset int) ([]Task, error)
	CountTasks(ctx context.Context, actorID int64, isAdmin bool, status string) (int64, error)
	UpdateStatus(ctx context.Context, id int64, fromRevision int64, status string) (int64, error)
	ListPoints(ctx context.Context, taskID int64) ([]TaskPoint, error)
	FillPointSnapshots(ctx context.Context, taskID int64, snaps []PointSnapshot) error
}

// PointSnapshot 派发时写入任务点快照的数据。
type PointSnapshot struct {
	PointID   int64
	Name      string
	SegmentID int64
	Latitude  float64
	Longitude float64
	RadiusM   float64
	Hint      string
}

// ListInput 列表查询条件。
type ListInput struct {
	Status   string
	Page     int
	PageSize int
}

// ListResult 分页列表结果。
type ListResult struct {
	Tasks []Task
	Total int64
}

// Service 任务业务规则。
type Service interface {
	// Create 创建草稿任务（管理员）。
	Create(ctx context.Context, actor *middleware.Actor, in CreateInput) (*Task, error)
	// Dispatch 派发任务：DRAFT → ASSIGNED，写点位快照（管理员）。
	Dispatch(ctx context.Context, actor *middleware.Actor, taskID, revision int64) (*Task, error)
	// List 按权限返回任务列表（支持状态过滤 + 分页）。
	List(ctx context.Context, actor *middleware.Actor, in ListInput) (*ListResult, error)
	// Get 返回任务与任务点。
	Get(ctx context.Context, actor *middleware.Actor, id int64) (*Task, []TaskPoint, error)
	// SubmitTask 提交整任务：ASSIGNED/IN_PROGRESS → PENDING_REVIEW（负责人）。
	SubmitTask(ctx context.Context, actor *middleware.Actor, taskID, revision int64) (*Task, error)
	// Complete 完成任务：PENDING_REVIEW → COMPLETED（审核员）。
	Complete(ctx context.Context, actor *middleware.Actor, taskID, revision int64, finalNote string) (*Task, error)
}

type service struct {
	repo        Repository
	obsStatus   ObservationStatusProvider
	pointGetter func(ctx context.Context, pointID int64) (PointSnapshot, error)
	audit       audit.Recorder
}

// NewService 构造任务服务。
// obs 为任务点记录状态提供者；pointGetter 由 site 模块提供（派发时读取点位当前信息）；
// rec 为审计记录器（可为 no-op，审计失败不阻断业务）。
func NewService(
	repo Repository,
	obs ObservationStatusProvider,
	pointGetter func(ctx context.Context, pointID int64) (PointSnapshot, error),
	rec audit.Recorder,
) Service {
	return &service{repo: repo, obsStatus: obs, pointGetter: pointGetter, audit: rec}
}

func (s *service) Create(ctx context.Context, actor *middleware.Actor, in CreateInput) (*Task, error) {
	if !hasRole(actor, "ADMIN") {
		return nil, ErrForbidden
	}
	if len(in.PointIDs) == 0 {
		return nil, ErrNoPoints
	}
	seen := map[int64]bool{}
	for _, pid := range in.PointIDs {
		if seen[pid] {
			return nil, ErrDuplicatePoint
		}
		seen[pid] = true
		if _, err := s.pointGetter(ctx, pid); err != nil {
			return nil, ErrPointNotFound
		}
	}
	t := &Task{
		Title:       in.Title,
		AssigneeID:  in.AssigneeID,
		DueAt:       in.DueAt,
		Description: in.Description,
		Status:      StatusDraft,
		Revision:    1,
	}
	if err := s.repo.InsertTask(ctx, t); err != nil {
		return nil, err
	}
	if err := s.repo.InsertTaskPoints(ctx, t.ID, in.PointIDs); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *service) Dispatch(ctx context.Context, actor *middleware.Actor, taskID, revision int64) (*Task, error) {
	if !hasRole(actor, "ADMIN") {
		return nil, ErrForbidden
	}
	t, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return nil, ErrNotFound
	}
	if t.Status != StatusDraft {
		return nil, ErrInvalidState
	}
	if t.Revision != revision {
		return nil, ErrStatusConflict
	}
	points, err := s.repo.ListPoints(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if len(points) == 0 {
		return nil, ErrNoPoints
	}
	snaps := make([]PointSnapshot, 0, len(points))
	for _, p := range points {
		snap, err := s.pointGetter(ctx, p.PointID)
		if err != nil {
			return nil, ErrPointNotFound
		}
		snaps = append(snaps, snap)
	}
	if err := s.repo.FillPointSnapshots(ctx, taskID, snaps); err != nil {
		return nil, err
	}
	n, err := s.repo.UpdateStatus(ctx, taskID, revision, StatusAssigned)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrStatusConflict
	}
	now := time.Now().UTC()
	t.Status = StatusAssigned
	t.Revision++
	t.DispatchedAt = &now
	s.audit.Record(ctx, &audit.Event{
		EntityType: "task",
		EntityID:   t.ID,
		Action:     "dispatch",
		ActorID:    actor.UserID,
		OldStatus:  StatusDraft,
		NewStatus:  StatusAssigned,
		Revision:   &t.Revision,
	})
	return t, nil
}

func (s *service) List(ctx context.Context, actor *middleware.Actor, in ListInput) (*ListResult, error) {
	isAdmin := hasRole(actor, "ADMIN") || hasRole(actor, "REVIEWER")
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
	tasks, err := s.repo.ListPage(ctx, actor.UserID, isAdmin, in.Status, limit, offset)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountTasks(ctx, actor.UserID, isAdmin, in.Status)
	if err != nil {
		return nil, err
	}
	return &ListResult{Tasks: tasks, Total: total}, nil
}

func (s *service) Get(ctx context.Context, actor *middleware.Actor, id int64) (*Task, []TaskPoint, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, ErrNotFound
	}
	if !(hasRole(actor, "ADMIN") || hasRole(actor, "REVIEWER") || t.AssigneeID == actor.UserID) {
		return nil, nil, ErrForbidden
	}
	points, err := s.repo.ListPoints(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return t, points, nil
}

func (s *service) SubmitTask(ctx context.Context, actor *middleware.Actor, taskID, revision int64) (*Task, error) {
	t, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return nil, ErrNotFound
	}
	if t.AssigneeID != actor.UserID && !hasRole(actor, "ADMIN") {
		return nil, ErrForbidden
	}
	if t.Status != StatusAssigned && t.Status != StatusInProgress {
		return nil, ErrInvalidState
	}
	if t.Revision != revision {
		return nil, ErrStatusConflict
	}
	statuses, err := s.obsStatus.TaskPointStatuses(ctx, taskID)
	if err != nil {
		return nil, err
	}
	points, err := s.repo.ListPoints(ctx, taskID)
	if err != nil {
		return nil, err
	}
	for _, p := range points {
		st, ok := statuses[p.ID]
		if !ok {
			return nil, ErrUnfinishedPoints
		}
		if st != "SUBMITTED" && st != "ACCEPTED" {
			return nil, ErrUnfinishedPoints
		}
	}
	n, err := s.repo.UpdateStatus(ctx, taskID, revision, StatusPendingReview)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrStatusConflict
	}
	t.Status = StatusPendingReview
	t.Revision++
	s.audit.Record(ctx, &audit.Event{
		EntityType: "task",
		EntityID:   t.ID,
		Action:     "submit_task",
		ActorID:    actor.UserID,
		OldStatus:  StatusAssigned,
		NewStatus:  StatusPendingReview,
		Revision:   &t.Revision,
	})
	return t, nil
}

func (s *service) Complete(ctx context.Context, actor *middleware.Actor, taskID, revision int64, finalNote string) (*Task, error) {
	if !hasRole(actor, "REVIEWER") && !hasRole(actor, "ADMIN") {
		return nil, ErrForbidden
	}
	t, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return nil, ErrNotFound
	}
	if t.Status != StatusPendingReview {
		return nil, ErrInvalidState
	}
	if t.Revision != revision {
		return nil, ErrStatusConflict
	}
	statuses, err := s.obsStatus.TaskPointStatuses(ctx, taskID)
	if err != nil {
		return nil, err
	}
	points, err := s.repo.ListPoints(ctx, taskID)
	if err != nil {
		return nil, err
	}
	for _, p := range points {
		st, ok := statuses[p.ID]
		if !ok {
			return nil, ErrUnfinishedPoints
		}
		if st != "ACCEPTED" {
			return nil, ErrUnfinishedPoints
		}
	}
	now := time.Now().UTC()
	n, err := s.repo.UpdateStatus(ctx, taskID, revision, StatusCompleted)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrStatusConflict
	}
	t.Status = StatusCompleted
	t.Revision++
	t.CompletedAt = &now
	s.audit.Record(ctx, &audit.Event{
		EntityType: "task",
		EntityID:   t.ID,
		Action:     "complete",
		ActorID:    actor.UserID,
		OldStatus:  StatusPendingReview,
		NewStatus:  StatusCompleted,
		Revision:   &t.Revision,
		Reason:     finalNote,
	})
	return t, nil
}

func hasRole(actor *middleware.Actor, role string) bool {
	if actor == nil {
		return false
	}
	for _, r := range actor.Roles {
		if r == role {
			return true
		}
	}
	return false
}
