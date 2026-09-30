package audit

import (
	"context"
	"log"

	"gorm.io/gorm"
)

// GORMRepository 基于 GORM 的实现。
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository 构造审计仓储。
func NewGORMRepository(db *gorm.DB) Repository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) Insert(ctx context.Context, e *Event) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *GORMRepository) ListEvents(ctx context.Context, f EventFilter, limit, offset int) ([]Event, int64, error) {
	q := r.db.WithContext(ctx).Model(&Event{})
	if f.EntityType != "" {
		q = q.Where("entity_type = ?", f.EntityType)
	}
	if f.EntityID != nil {
		q = q.Where("entity_id = ?", *f.EntityID)
	}
	if f.ActorID != nil {
		q = q.Where("actor_id = ?", *f.ActorID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Event
	if err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// noopRecorder 数据库未就绪（dev 降级）时的空实现，不阻断业务。
type noopRecorder struct{}

func (noopRecorder) Record(_ context.Context, _ *Event) {}

// RecorderImpl 落库实现的审计记录器。
type RecorderImpl struct {
	repo Repository
}

func (r *RecorderImpl) Record(ctx context.Context, e *Event) {
	if r == nil || r.repo == nil {
		return
	}
	if err := r.repo.Insert(ctx, e); err != nil {
		// 审计失败不阻断业务，仅记录日志。
		log.Printf("[audit] insert event failed: %v (type=%s id=%d action=%s)", err, e.EntityType, e.EntityID, e.Action)
	}
}

// NewRecorder 构造审计记录器；db 为 nil 时返回 no-op（dev 降级）。
func NewRecorder(db *gorm.DB) Recorder {
	if db == nil {
		return noopRecorder{}
	}
	return &RecorderImpl{repo: NewGORMRepository(db)}
}
