package audit

import (
	"context"
	"errors"
)

// ErrForbidden 无查询权限（仅管理员）。
var ErrForbidden = errors.New("audit: forbidden")

// ErrDBUnavailable 数据库未就绪。
var ErrDBUnavailable = errors.New("audit: database unavailable")

// QueryService 审计查询服务（只读，仅管理员）。
type QueryService struct {
	repo Repository
}

// NewQueryService 构造审计查询服务。
func NewQueryService(repo Repository) *QueryService {
	return &QueryService{repo: repo}
}

// Page 审计分页结果。
type Page struct {
	Items []Event `json:"items"`
	Total int64   `json:"total"`
}

// List 分页查询审计事件；actorIsAdmin 非真时返回 ErrForbidden。
func (s *QueryService) List(ctx context.Context, f EventFilter, page, pageSize int, actorIsAdmin bool) (*Page, error) {
	if !actorIsAdmin {
		return nil, ErrForbidden
	}
	if s == nil || s.repo == nil {
		return nil, ErrDBUnavailable
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	items, total, err := s.repo.ListEvents(ctx, f, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	return &Page{Items: items, Total: total}, nil
}
