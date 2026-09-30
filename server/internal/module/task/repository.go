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
// 返回具体类型：observation 模块需使用其 GetTaskPoint/MarkInProgressIfAssigned/ListPoints 能力。
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) InsertTask(ctx context.Context, t *Task) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *GORMRepository) InsertTaskPoints(ctx context.Context, taskID int64, pointIDs []int64) error {
	if len(pointIDs) == 0 {
		return nil
	}
	points := make([]TaskPoint, 0, len(pointIDs))
	for i, pid := range pointIDs {
		points = append(points, TaskPoint{
			TaskID:     taskID,
			PointID:    pid,
			SequenceNo: i + 1,
		})
	}
	return r.db.WithContext(ctx).Create(&points).Error
}

func (r *GORMRepository) GetByID(ctx context.Context, id int64) (*Task, error) {
	var t Task
	if err := r.db.WithContext(ctx).First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *GORMRepository) ListByActor(ctx context.Context, actorID int64, isAdmin bool) ([]Task, error) {
	var tasks []Task
	q := r.db.WithContext(ctx).Order("created_at DESC")
	if !isAdmin {
		q = q.Where("assignee_id = ?", actorID)
	}
	return tasks, q.Find(&tasks).Error
}

// ListPage 分页 + 状态过滤查询（offset/limit 为 0 时不分页）。
func (r *GORMRepository) ListPage(ctx context.Context, actorID int64, isAdmin bool, status string, limit, offset int) ([]Task, error) {
	var tasks []Task
	q := r.db.WithContext(ctx).Order("created_at DESC")
	if !isAdmin {
		q = q.Where("assignee_id = ?", actorID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if limit > 0 {
		q = q.Limit(limit)
		if offset > 0 {
			q = q.Offset(offset)
		}
	}
	return tasks, q.Find(&tasks).Error
}

// CountTasks 统计任务数（与 ListPage 同一过滤条件）。
func (r *GORMRepository) CountTasks(ctx context.Context, actorID int64, isAdmin bool, status string) (int64, error) {
	var total int64
	q := r.db.WithContext(ctx).Model(&Task{})
	if !isAdmin {
		q = q.Where("assignee_id = ?", actorID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	return total, q.Count(&total).Error
}

func (r *GORMRepository) UpdateStatus(ctx context.Context, id int64, fromRevision int64, status string) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&Task{}).
		Where("id = ? AND revision = ?", id, fromRevision).
		Updates(map[string]any{
			"status":   status,
			"revision": gorm.Expr("revision + 1"),
		})
	return res.RowsAffected, res.Error
}

func (r *GORMRepository) ListPoints(ctx context.Context, taskID int64) ([]TaskPoint, error) {
	var pts []TaskPoint
	return pts, r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("sequence_no").
		Find(&pts).Error
}

func (r *GORMRepository) FillPointSnapshots(ctx context.Context, taskID int64, snaps []PointSnapshot) error {
	if len(snaps) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, s := range snaps {
			name, segID, lat, lng, radius, hint := s.Name, s.SegmentID, s.Latitude, s.Longitude, s.RadiusM, s.Hint
			res := tx.Model(&TaskPoint{}).
				Where("task_id = ? AND point_id = ?", taskID, s.PointID).
				Updates(map[string]any{
					"snapshot_name":          &name,
					"snapshot_segment_id":    &segID,
					"snapshot_latitude":      &lat,
					"snapshot_longitude":     &lng,
					"snapshot_radius_m":      &radius,
					"snapshot_location_hint": &hint,
				})
			if res.Error != nil {
				return res.Error
			}
		}
		return nil
	})
}

// MarkInProgressIfAssigned 实现 observation.TaskLifecycle：
// 首次现场采集证据保存时，任务由 ASSIGNED 转为 IN_PROGRESS（不阻断业务）。
func (r *GORMRepository) MarkInProgressIfAssigned(ctx context.Context, taskID int64) error {
	res := r.db.WithContext(ctx).
		Model(&Task{}).
		Where("id = ? AND status = ?", taskID, StatusAssigned).
		Update("status", StatusInProgress)
	return res.Error
}

// GetTaskPoint 实现 observation.TaskPointRepository：按 ID 查询任务点。
func (r *GORMRepository) GetTaskPoint(ctx context.Context, id int64) (*TaskPoint, error) {
	var p TaskPoint
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}
