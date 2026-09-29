package task

import (
	"context"

	"gorm.io/gorm"
)

// GORMRepository 基于 GORM 的 Repository 实现。
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository 构造任务仓储。
func NewGORMRepository(db *gorm.DB) Repository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) Insert(ctx context.Context, t *Task, points []TaskPoint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return err
		}
		if len(points) > 0 {
			return tx.Create(&points).Error
		}
		return nil
	})
}

func (r *GORMRepository) GetByID(ctx context.Context, id int64) (*Task, error) {
	var t Task
	if err := r.db.WithContext(ctx).First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *GORMRepository) UpdateStatus(ctx context.Context, id int64, fromRevision int64, status string) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&Task{}).
		Where("id = ? AND revision = ?", id, fromRevision).
		Updates(map[string]any{"status": status, "revision": gorm.Expr("revision + 1")})
	return res.RowsAffected, res.Error
}
