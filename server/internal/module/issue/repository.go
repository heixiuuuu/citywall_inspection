package issue

import (
	"context"

	"gorm.io/gorm"
)

// Repository 问题数据访问接口。
type Repository interface {
	Insert(ctx context.Context, i *Issue) error
	GetByID(ctx context.Context, id int64) (*Issue, error)
	ListPage(ctx context.Context, actorID int64, isAdmin bool, status string, limit, offset int) ([]Issue, error)
	CountIssues(ctx context.Context, actorID int64, isAdmin bool, status string) (int64, error)

	UpdateStatus(ctx context.Context, id, fromRevision int64, status string) (int64, error)
	UpdateAssignee(ctx context.Context, id, fromRevision int64, assigneeID int64) (int64, error)

	InsertAction(ctx context.Context, a *IssueAction) error
	ListActions(ctx context.Context, issueID int64) ([]IssueAction, error)
	LinkObservation(ctx context.Context, link *IssueObservation) error
	ListObservations(ctx context.Context, issueID int64) ([]IssueObservation, error)
	ListByObservation(ctx context.Context, observationID int64) ([]IssueObservation, error)
	LinkActionImage(ctx context.Context, actionID, imageID int64) error

	// InTx 在同一事务中执行 fn（用于受理等原子写入）。
	InTx(ctx context.Context, fn func(Repository) error) error
}

// GORMRepository 基于 GORM 的实现。
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository 构造问题仓储。
func NewGORMRepository(db *gorm.DB) Repository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) Insert(ctx context.Context, i *Issue) error {
	return r.db.WithContext(ctx).Create(i).Error
}

func (r *GORMRepository) GetByID(ctx context.Context, id int64) (*Issue, error) {
	var i Issue
	if err := r.db.WithContext(ctx).First(&i, id).Error; err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *GORMRepository) ListPage(ctx context.Context, actorID int64, isAdmin bool, status string, limit, offset int) ([]Issue, error) {
	var items []Issue
	q := r.db.WithContext(ctx).Order("opened_at DESC")
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
	return items, q.Find(&items).Error
}

func (r *GORMRepository) CountIssues(ctx context.Context, actorID int64, isAdmin bool, status string) (int64, error) {
	var total int64
	q := r.db.WithContext(ctx).Model(&Issue{})
	if !isAdmin {
		q = q.Where("assignee_id = ?", actorID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	return total, q.Count(&total).Error
}

func (r *GORMRepository) UpdateStatus(ctx context.Context, id, fromRevision int64, status string) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&Issue{}).
		Where("id = ? AND revision = ?", id, fromRevision).
		Updates(map[string]any{"status": status, "revision": gorm.Expr("revision + 1")})
	return res.RowsAffected, res.Error
}

func (r *GORMRepository) UpdateAssignee(ctx context.Context, id, fromRevision int64, assigneeID int64) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&Issue{}).
		Where("id = ? AND revision = ?", id, fromRevision).
		Updates(map[string]any{"assignee_id": assigneeID, "revision": gorm.Expr("revision + 1")})
	return res.RowsAffected, res.Error
}

func (r *GORMRepository) InsertAction(ctx context.Context, a *IssueAction) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *GORMRepository) ListActions(ctx context.Context, issueID int64) ([]IssueAction, error) {
	var items []IssueAction
	return items, r.db.WithContext(ctx).
		Where("issue_id = ?", issueID).
		Order("created_at ASC, id ASC").
		Find(&items).Error
}

func (r *GORMRepository) LinkObservation(ctx context.Context, link *IssueObservation) error {
	return r.db.WithContext(ctx).Create(link).Error
}

func (r *GORMRepository) ListObservations(ctx context.Context, issueID int64) ([]IssueObservation, error) {
	var items []IssueObservation
	return items, r.db.WithContext(ctx).
		Where("issue_id = ?", issueID).
		Order("linked_at ASC").
		Find(&items).Error
}

func (r *GORMRepository) ListByObservation(ctx context.Context, observationID int64) ([]IssueObservation, error) {
	var items []IssueObservation
	return items, r.db.WithContext(ctx).
		Where("observation_id = ?", observationID).
		Order("linked_at ASC").
		Find(&items).Error
}

func (r *GORMRepository) LinkActionImage(ctx context.Context, actionID, imageID int64) error {
	return r.db.WithContext(ctx).Create(&IssueActionImage{
		IssueActionID: actionID,
		ImageID:       imageID,
	}).Error
}

// InTx 在同一事务中执行 fn；事务内写入使用 tx 版仓储。
func (r *GORMRepository) InTx(ctx context.Context, fn func(Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&GORMRepository{db: tx})
	})
}
