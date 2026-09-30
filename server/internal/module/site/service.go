// Package site 承载城墙区段与巡检点位维护。
// 点位停用不删除历史；定位半径是提示阈值，异常交人工审核。
package site

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrPointNotFound 点位不存在。
	ErrPointNotFound = errors.New("site: inspection point not found")
	// ErrSegmentNotFound 区段不存在。
	ErrSegmentNotFound = errors.New("site: wall segment not found")
)

// WallSegment 城墙区段。
type WallSegment struct {
	ID          int64     `gorm:"primaryKey;column:id" json:"id"`
	Code        string    `gorm:"column:code;uniqueIndex" json:"code"`
	Name        string    `gorm:"column:name" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
	Active      bool      `gorm:"column:active" json:"active"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// InspectionPoint 巡检点位。
type InspectionPoint struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	SegmentID    int64     `gorm:"column:segment_id;index" json:"segment_id"`
	Code         string    `gorm:"column:code;uniqueIndex" json:"code"`
	Name         string    `gorm:"column:name" json:"name"`
	Latitude     float64   `gorm:"column:latitude" json:"latitude"`
	Longitude    float64   `gorm:"column:longitude" json:"longitude"`
	RadiusM      float64   `gorm:"column:radius_m" json:"radius_m"`
	LocationHint string    `gorm:"column:location_hint" json:"location_hint"`
	Active       bool      `gorm:"column:active" json:"active"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 显式表名。
func (WallSegment) TableName() string     { return "wall_segments" }
func (InspectionPoint) TableName() string { return "inspection_points" }

// CreatePointInput 创建点位入参。
type CreatePointInput struct {
	SegmentID    int64   `json:"segment_id" binding:"required"`
	Code         string  `json:"code" binding:"required"`
	Name         string  `json:"name" binding:"required"`
	Latitude     float64 `json:"latitude" binding:"required"`
	Longitude    float64 `json:"longitude" binding:"required"`
	RadiusM      float64 `json:"radius_m"`
	LocationHint string  `json:"location_hint"`
}

// Repository 点位数据访问。
type Repository interface {
	ListPoints(ctx context.Context, onlyActive bool) ([]InspectionPoint, error)
	GetPoint(ctx context.Context, id int64) (*InspectionPoint, error)
	CreatePoint(ctx context.Context, p *InspectionPoint) error
	CreateSegment(ctx context.Context, s *WallSegment) error
	ListSegments(ctx context.Context, onlyActive bool) ([]WallSegment, error)
}

// Service 点位维护规则（管理员维护；巡检端只读）。
type Service interface {
	ListPoints(ctx context.Context, onlyActive bool) ([]InspectionPoint, error)
	ListSegments(ctx context.Context, onlyActive bool) ([]WallSegment, error)
	CreatePoint(ctx context.Context, in CreatePointInput) (*InspectionPoint, error)
}

type service struct {
	repo Repository
}

// NewService 构造点位服务。
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ListPoints(ctx context.Context, onlyActive bool) ([]InspectionPoint, error) {
	return s.repo.ListPoints(ctx, onlyActive)
}

func (s *service) ListSegments(ctx context.Context, onlyActive bool) ([]WallSegment, error) {
	return s.repo.ListSegments(ctx, onlyActive)
}

func (s *service) CreatePoint(ctx context.Context, in CreatePointInput) (*InspectionPoint, error) {
	p := &InspectionPoint{
		SegmentID:    in.SegmentID,
		Code:         in.Code,
		Name:         in.Name,
		Latitude:     in.Latitude,
		Longitude:    in.Longitude,
		RadiusM:      in.RadiusM,
		LocationHint: in.LocationHint,
		Active:       true,
	}
	if err := s.repo.CreatePoint(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}
