package observation

import (
	"context"
	"errors"
)

var (
	// ErrNotImplemented 骨架阶段占位。
	ErrNotImplemented = errors.New("observation: not implemented yet")
)

// Repository 现场记录数据访问接口。
type Repository interface {
	// InsertDraft 创建草稿记录。
	InsertDraft(ctx context.Context, o *Observation) error
	// GetByID 按 ID 查询。
	GetByID(ctx context.Context, id int64) (*Observation, error)
}

// Service 现场记录业务规则。
type Service interface {
	// Create 创建草稿：source=TASK 必须带 task_point_id；source=VOLUNTARY 必须为空且带位置描述。
	Create(ctx context.Context, actorID int64, source string, taskPointID *int64, locationText string) (*Observation, error)
}

type service struct {
	repo Repository
}

// NewService 构造现场记录服务。
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, actorID int64, source string, taskPointID *int64, locationText string) (*Observation, error) {
	// TODO(init): 校验来源约束；任务点只允许一条逻辑 Observation；创建 DRAFT。
	return nil, ErrNotImplemented
}
