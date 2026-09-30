package audit

import (
	"context"

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
