package site

import (
	"context"

	"gorm.io/gorm"
)

// GORMRepository 基于 GORM 的实现。
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository 构造点位仓储。
func NewGORMRepository(db *gorm.DB) Repository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) ListPoints(ctx context.Context, onlyActive bool) ([]InspectionPoint, error) {
	var pts []InspectionPoint
	q := r.db.WithContext(ctx).Order("inspection_points.segment_id, inspection_points.code")
	if onlyActive {
		q = q.Select("inspection_points.*").Joins("JOIN wall_segments ON wall_segments.id = inspection_points.segment_id").
			Where("inspection_points.active = ? AND wall_segments.active = ?", true, true)
	}
	return pts, q.Find(&pts).Error
}

func (r *GORMRepository) GetPoint(ctx context.Context, id int64) (*InspectionPoint, error) {
	var p InspectionPoint
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *GORMRepository) CreatePoint(ctx context.Context, p *InspectionPoint) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *GORMRepository) GetSegment(ctx context.Context, id int64) (*WallSegment, error) {
	var segment WallSegment
	if err := r.db.WithContext(ctx).First(&segment, id).Error; err != nil {
		return nil, err
	}
	return &segment, nil
}
func (r *GORMRepository) UpdateSegment(ctx context.Context, segment *WallSegment) error {
	return r.db.WithContext(ctx).Save(segment).Error
}
func (r *GORMRepository) UpdatePoint(ctx context.Context, point *InspectionPoint) error {
	return r.db.WithContext(ctx).Save(point).Error
}

func (r *GORMRepository) CreateSegment(ctx context.Context, s *WallSegment) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *GORMRepository) ListSegments(ctx context.Context, onlyActive bool) ([]WallSegment, error) {
	var segs []WallSegment
	q := r.db.WithContext(ctx).Order("code")
	if onlyActive {
		q = q.Where("active = ?", true)
	}
	return segs, q.Find(&segs).Error
}
