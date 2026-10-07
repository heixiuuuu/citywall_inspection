package idempotency

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// Repository 幂等记录数据访问。
type Repository interface {
	// Get 按唯一键查询已有记录。
	Get(ctx context.Context, actorID int64, operation, idemKey string) (*Record, error)
	// Save 写入记录；唯一键冲突返回 ErrRecordExists（并发重放）。
	Save(ctx context.Context, r *Record) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository 构造幂等仓储。
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Get(ctx context.Context, actorID int64, operation, idemKey string) (*Record, error) {
	var rec Record
	err := r.db.WithContext(ctx).
		Where("actor_id = ? AND operation = ? AND idem_key = ?", actorID, operation, idemKey).
		First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *repository) Save(ctx context.Context, rec *Record) error {
	err := r.db.WithContext(ctx).Create(rec).Error
	if err != nil && isDuplicateKey(err) {
		return ErrRecordExists
	}
	return err
}

// isDuplicateKey 判断 MySQL 唯一键冲突错误。
func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := err.Error()
	return contains(msg, "Duplicate entry") || contains(msg, "Error 1062")
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
