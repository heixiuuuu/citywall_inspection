package task

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GORMRepository 基于 GORM 的 Repository 实现。
type GORMRepository struct {
	db     *gorm.DB
	locked bool
}

func (r *GORMRepository) Transaction(ctx context.Context, operation func(Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return operation(&GORMRepository{db: tx, locked: true})
	})
}

// NewGORMRepository 构造任务仓储。
// 返回具体类型：observation 模块需使用其 GetTaskPoint/ListPoints 能力。
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) ListAssignees(ctx context.Context) ([]Assignee, error) {
	users := make([]Assignee, 0)
	err := r.db.WithContext(ctx).Table("users").Select("users.id, users.display_name").
		Joins("JOIN user_roles ON user_roles.user_id = users.id").
		Where("users.status = ? AND user_roles.role_code = ?", "ACTIVE", "INSPECTOR").
		Order("users.display_name, users.id").Scan(&users).Error
	return users, err
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
	query := r.db.WithContext(ctx)
	if r.locked {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&t, id).Error; err != nil {
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
func (r *GORMRepository) ListPage(ctx context.Context, actorID int64, isAdmin bool, filter ListFilter, limit, offset int) ([]Task, error) {
	var tasks []Task
	q := r.db.WithContext(ctx).Order("created_at DESC, id DESC")
	if !isAdmin {
		q = q.Where("assignee_id = ?", actorID)
	}
	q = applyListFilter(q, filter)
	if limit > 0 {
		q = q.Limit(limit)
		if offset > 0 {
			q = q.Offset(offset)
		}
	}
	return tasks, q.Find(&tasks).Error
}

// CountTasks 统计任务数（与 ListPage 同一过滤条件）。
func (r *GORMRepository) CountTasks(ctx context.Context, actorID int64, isAdmin bool, filter ListFilter) (int64, error) {
	var total int64
	q := r.db.WithContext(ctx).Model(&Task{})
	if !isAdmin {
		q = q.Where("assignee_id = ?", actorID)
	}
	q = applyListFilter(q, filter)
	return total, q.Count(&total).Error
}

func applyListFilter(q *gorm.DB, filter ListFilter) *gorm.DB {
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.Keyword != "" {
		q = q.Where("LOCATE(?, title) > 0", filter.Keyword)
	}
	if filter.DueBefore != nil {
		q = q.Where("due_at < ? AND status IN ?", *filter.DueBefore, []string{StatusAssigned, StatusInProgress, StatusReturned, StatusPendingReview})
	}
	return q
}

func (r *GORMRepository) UpdateStatus(ctx context.Context, id int64, fromRevision int64, status string) (int64, error) {
	changes := map[string]any{"status": status, "revision": gorm.Expr("revision + 1")}
	if status == StatusAssigned {
		changes["dispatched_at"] = time.Now().UTC()
	}
	if status == StatusCompleted {
		changes["completed_at"] = time.Now().UTC()
	}
	res := r.db.WithContext(ctx).
		Model(&Task{}).
		Where("id = ? AND revision = ?", id, fromRevision).
		Updates(changes)
	return res.RowsAffected, res.Error
}

// CancelTask 取消任务：仅非终态可取消，写入取消理由与时间；已取消的幂等返回影响行数 0（由 service 判断现状）。
func (r *GORMRepository) CancelTask(ctx context.Context, id int64, fromRevision int64, reason string) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&Task{}).
		Where("id = ? AND revision = ? AND status IN ?", id, fromRevision, []string{StatusDraft, StatusAssigned, StatusInProgress}).
		Updates(map[string]any{
			"status":        StatusCancelled,
			"cancel_reason": reason,
			"cancelled_at":  gorm.Expr("NOW()"),
			"revision":      gorm.Expr("revision + 1"),
		})
	return res.RowsAffected, res.Error
}

func (r *GORMRepository) ListPoints(ctx context.Context, taskID int64) ([]TaskPoint, error) {
	pts := []TaskPoint{}
	err := r.db.WithContext(ctx).
		Select("task_points.*, COALESCE(p.name, '') AS point_name, o.id AS observation_id, o.reporter_id AS observation_reporter_id, COALESCE(o.status, '') AS observation_status, COALESCE(o.inspection_outcome, '') AS observation_outcome").
		Joins("LEFT JOIN inspection_points p ON p.id = task_points.point_id").
		Joins("LEFT JOIN observations o ON o.task_point_id = task_points.id").
		Where("task_points.task_id = ?", taskID).
		Order("task_points.sequence_no").
		Find(&pts).Error
	return pts, err
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

// GetTaskPoint 实现 observation.TaskPointRepository：按 ID 查询任务点。
func (r *GORMRepository) GetTaskPoint(ctx context.Context, id int64) (*TaskPoint, error) {
	var p TaskPoint
	if err := r.db.WithContext(ctx).Table("task_points").
		Select("task_points.*, tasks.assignee_id, tasks.status AS task_status").
		Joins("JOIN tasks ON tasks.id = task_points.task_id").
		Where("task_points.id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}
