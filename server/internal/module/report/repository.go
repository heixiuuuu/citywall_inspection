package report

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	// ErrNotFound 报告不存在。
	ErrNotFound = errors.New("report: not found")
	// ErrForbidden 无权限访问报告。
	ErrForbidden = errors.New("report: forbidden")
	// ErrInvalidScope 报告范围不合法。
	ErrInvalidScope = errors.New("report: invalid scope")
	// ErrNotReady PDF 尚未生成完成。
	ErrNotReady = errors.New("report: not ready")
	// ErrBadState 重试/状态操作不合法（如非 FAILED 重试）。
	ErrBadState = errors.New("report: invalid state for operation")
)

// Repository 报告数据访问。
type Repository interface {
	// Insert 创建报告记录（GENERATING），返回 ID。
	Insert(ctx context.Context, r *Report) error
	// GetByID 按 ID 读取。
	GetByID(ctx context.Context, id int64) (*Report, error)
	// ListPage 台账分页。
	ListPage(ctx context.Context, scopeType string, taskID *int64, pageSize, offset int) ([]Report, error)
	// Count 台账总数。
	Count(ctx context.Context, scopeType string, taskID *int64) (int64, error)
	// MarkReady 条件更新为 READY（携带 file_key、snapshot_json 与完成时间）。
	MarkReady(ctx context.Context, id int64, fileKey, snapshotJSON string) error
	// MarkFailed 条件更新为 FAILED（携带错误码）。
	MarkFailed(ctx context.Context, id int64, errorCode string) error
	// MarkGenerating 仅 FAILED 记录可重试：重置为 GENERATING。
	MarkGenerating(ctx context.Context, id int64) (int64, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository 构造报告仓储。
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Insert(ctx context.Context, rep *Report) error {
	return r.db.WithContext(ctx).Create(rep).Error
}

func (r *repository) GetByID(ctx context.Context, id int64) (*Report, error) {
	var rep Report
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&rep).Error
	if err == gorm.ErrRecordNotFound {
		return nil, ErrNotFound
	}
	return &rep, err
}

func (r *repository) ListPage(ctx context.Context, scopeType string, taskID *int64, pageSize, offset int) ([]Report, error) {
	q := r.db.WithContext(ctx).Model(&Report{})
	if scopeType != "" {
		q = q.Where("scope_type = ?", scopeType)
	}
	if taskID != nil {
		q = q.Where("task_id = ?", *taskID)
	}
	var items []Report
	err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error
	return items, err
}

func (r *repository) Count(ctx context.Context, scopeType string, taskID *int64) (int64, error) {
	q := r.db.WithContext(ctx).Model(&Report{})
	if scopeType != "" {
		q = q.Where("scope_type = ?", scopeType)
	}
	if taskID != nil {
		q = q.Where("task_id = ?", *taskID)
	}
	var n int64
	err := q.Count(&n).Error
	return n, err
}

func (r *repository) MarkReady(ctx context.Context, id int64, fileKey, snapshotJSON string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&Report{}).
		Where("id = ? AND status = ?", id, StatusGenerating).
		Updates(map[string]any{"status": StatusReady, "file_key": fileKey, "snapshot_json": snapshotJSON, "finished_at": now}).Error
}

func (r *repository) MarkFailed(ctx context.Context, id int64, errorCode string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&Report{}).
		Where("id = ? AND status = ?", id, StatusGenerating).
		Updates(map[string]any{"status": StatusFailed, "error_code": errorCode, "finished_at": now}).Error
}

func (r *repository) MarkGenerating(ctx context.Context, id int64) (int64, error) {
	res := r.db.WithContext(ctx).Model(&Report{}).
		Where("id = ? AND status = ?", id, StatusFailed).
		Updates(map[string]any{"status": StatusGenerating, "error_code": "", "finished_at": nil})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
