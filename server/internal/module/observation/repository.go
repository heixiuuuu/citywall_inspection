package observation

import (
	"context"

	"gorm.io/gorm"
)

// GORMRepository 基于 GORM 的 Repository 实现。
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository 构造现场记录仓储。
func NewGORMRepository(db *gorm.DB) Repository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) InsertDraft(ctx context.Context, o *Observation) error {
	return r.db.WithContext(ctx).Create(o).Error
}

func (r *GORMRepository) GetByID(ctx context.Context, id int64) (*Observation, error) {
	var o Observation
	if err := r.db.WithContext(ctx).First(&o, id).Error; err != nil {
		return nil, err
	}
	return &o, nil
}
