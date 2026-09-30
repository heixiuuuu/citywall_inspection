package identity

import (
	"context"

	"gorm.io/gorm"
)

// GORMRepository 基于 GORM 的实现。
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository 构造身份仓储。
func NewGORMRepository(db *gorm.DB) Repository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *GORMRepository) GetByWechatOpenid(ctx context.Context, openid string) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).Where("wechat_openid = ?", openid).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *GORMRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *GORMRepository) GetRoles(ctx context.Context, userID int64) ([]string, error) {
	var roles []string
	err := r.db.WithContext(ctx).
		Model(&UserRole{}).
		Where("user_id = ?", userID).
		Pluck("role_code", &roles).Error
	return roles, err
}
