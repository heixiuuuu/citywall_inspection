package observation

import (
	"context"

	"citywall/server/internal/module/task"
	"gorm.io/gorm/clause"
)

func collectableTask(ctx context.Context, repo Repository, o *Observation) (int64, error) {
	if o.Source != SourceTask {
		return 0, nil
	}
	if o.TaskPointID == nil {
		return 0, ErrSourceConstraint
	}
	id, status, err := repo.TaskState(ctx, *o.TaskPointID)
	if err != nil {
		return 0, err
	}
	switch status {
	case task.StatusAssigned, task.StatusInProgress, task.StatusPendingReview, task.StatusReturned:
		return id, nil
	default:
		return 0, ErrInvalidState
	}
}

func (r *GORMRepository) TaskState(ctx context.Context, taskPointID int64) (int64, string, error) {
	var state struct {
		ID     int64
		Status string
	}
	query := r.db.WithContext(ctx).Table("tasks").Select("tasks.id, tasks.status").
		Joins("JOIN task_points ON task_points.task_id = tasks.id").Where("task_points.id = ?", taskPointID)
	if r.locking {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Take(&state).Error; err != nil {
		return 0, "", err
	}
	return state.ID, state.Status, nil
}

func (r *GORMRepository) MarkTaskInProgress(ctx context.Context, taskID int64) error {
	return r.db.WithContext(ctx).Table("tasks").Where("id = ? AND status = ?", taskID, task.StatusAssigned).
		Update("status", task.StatusInProgress).Error
}
