package task

import (
	"context"
	"errors"
)

var (
	// ErrNotImplemented 骨架阶段占位。
	ErrNotImplemented = errors.New("task: not implemented yet")
	// ErrStatusConflict 状态或 revision 冲突（对外映射 409）。
	ErrStatusConflict = errors.New("task: status or revision conflict")
)

// Repository 任务数据访问接口。
type Repository interface {
	// Insert 事务内写入任务与任务点快照。
	Insert(ctx context.Context, t *Task, points []TaskPoint) error
	// GetByID 按 ID 查询任务。
	GetByID(ctx context.Context, id int64) (*Task, error)
	// UpdateStatus 条件更新：WHERE id=? AND revision=?，返回影响行数。
	UpdateStatus(ctx context.Context, id int64, fromRevision int64, status string) (int64, error)
}

// Service 任务业务规则（创建、派发、提交、完成、取消的状态转移校验）。
type Service interface {
	// Create 创建草稿任务（管理员）。
	Create(ctx context.Context, actorID int64, title string, assigneeID int64, dueAt string, description string, pointIDs []int64) (*Task, error)
	// Dispatch 派发：DRAFT → ASSIGNED，写点位快照（管理员）。
	Dispatch(ctx context.Context, actorID, taskID, revision int64) (*Task, error)
}

type service struct {
	repo Repository
}

// NewService 构造任务服务。
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, actorID int64, title string, assigneeID int64, dueAt string, description string, pointIDs []int64) (*Task, error) {
	// TODO(init): 校验点位是否重复、是否存在；插入 DRAFT 任务。
	return nil, ErrNotImplemented
}

func (s *service) Dispatch(ctx context.Context, actorID, taskID, revision int64) (*Task, error) {
	// TODO(init): 校验管理员权限与 DRAFT 状态，事务内写任务点快照与审计事件。
	return nil, ErrNotImplemented
}
