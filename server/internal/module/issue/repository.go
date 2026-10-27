package issue

import (
	"citywall/server/internal/module/observation"
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository 问题数据访问接口。
type Repository interface {
	ListAssignees(ctx context.Context) ([]Assignee, error)
	Insert(ctx context.Context, i *Issue) error
	GetByID(ctx context.Context, id int64) (*Issue, error)
	ListPage(ctx context.Context, actorID int64, isAdmin bool, status string, limit, offset int) ([]Issue, error)
	CountIssues(ctx context.Context, actorID int64, isAdmin bool, status string) (int64, error)

	UpdateStatus(ctx context.Context, id, fromRevision int64, status string) (int64, error)
	Close(ctx context.Context, id, fromRevision int64, reason string) (int64, error)
	UpdateAssignee(ctx context.Context, id, fromRevision int64, assigneeID int64) (int64, error)

	InsertAction(ctx context.Context, a *IssueAction) error
	ListActions(ctx context.Context, issueID int64) ([]IssueAction, error)
	LinkObservation(ctx context.Context, link *IssueObservation) error
	ListObservationBriefs(ctx context.Context, issueID int64) ([]ObservationBrief, error)
	ListByObservation(ctx context.Context, observationID int64) ([]IssueObservation, error)
	LockObservation(ctx context.Context, observationID int64) (*observation.Observation, error)
	MarkAdmitted(ctx context.Context, observationID, actorID int64, reason string, at time.Time) error
	LinkActionImage(ctx context.Context, actionID, imageID int64) error

	// InTx 在同一事务中执行 fn（用于受理等原子写入）。
	InTx(ctx context.Context, fn func(Repository) error) error
}

type Assignee struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
}

// GORMRepository 基于 GORM 的实现。
type GORMRepository struct {
	db            *gorm.DB
	inTransaction bool
}

// NewGORMRepository 构造问题仓储。
func NewGORMRepository(db *gorm.DB) Repository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) ListAssignees(ctx context.Context) ([]Assignee, error) {
	users := []Assignee{}
	err := r.db.WithContext(ctx).Table("users").Select("DISTINCT users.id, users.display_name").
		Joins("JOIN user_roles ON user_roles.user_id = users.id").
		Where("users.status = ? AND user_roles.role_code IN ?", "ACTIVE", []string{"ADMIN", "REVIEWER", "INSPECTOR"}).
		Order("users.display_name, users.id").Scan(&users).Error
	return users, err
}

func (r *GORMRepository) Insert(ctx context.Context, i *Issue) error {
	return r.db.WithContext(ctx).Create(i).Error
}

func (r *GORMRepository) GetByID(ctx context.Context, id int64) (*Issue, error) {
	var i Issue
	query := r.db.WithContext(ctx)
	if r.inTransaction {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&i, id).Error; err != nil {
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
	updates := map[string]any{"status": status, "revision": gorm.Expr("revision + 1")}
	if status == StatusOpen {
		updates["closed_at"], updates["close_reason"] = nil, ""
	}
	res := r.db.WithContext(ctx).
		Model(&Issue{}).
		Where("id = ? AND revision = ?", id, fromRevision).
		Updates(updates)
	return res.RowsAffected, res.Error
}

func (r *GORMRepository) Close(ctx context.Context, id, fromRevision int64, reason string) (int64, error) {
	res := r.db.WithContext(ctx).Model(&Issue{}).Where("id = ? AND revision = ?", id, fromRevision).
		Updates(map[string]any{"status": StatusClosed, "revision": gorm.Expr("revision + 1"), "closed_at": time.Now().UTC(), "close_reason": reason})
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

func (r *GORMRepository) ListObservationBriefs(ctx context.Context, issueID int64) ([]ObservationBrief, error) {
	items := []ObservationBrief{}
	err := r.db.WithContext(ctx).Table("issue_observations AS link").
		Select("link.observation_id, link.link_reason, link.linked_at, o.source, o.inspection_outcome AS outcome, o.reporter_id, o.location_text, o.status").
		Joins("LEFT JOIN observations o ON o.id = link.observation_id").
		Where("link.issue_id = ?", issueID).Order("link.linked_at, link.observation_id").Scan(&items).Error
	return items, err
}

func (r *GORMRepository) ListByObservation(ctx context.Context, observationID int64) ([]IssueObservation, error) {
	var items []IssueObservation
	return items, r.db.WithContext(ctx).
		Where("observation_id = ?", observationID).
		Order("linked_at ASC").
		Find(&items).Error
}

func (r *GORMRepository) LockObservation(ctx context.Context, observationID int64) (*observation.Observation, error) {
	var record observation.Observation
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, observationID).Error
	return &record, err
}

func (r *GORMRepository) MarkAdmitted(ctx context.Context, observationID, actorID int64, reason string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&observation.Observation{}).Where("id = ?", observationID).
		Updates(map[string]any{"admitted_at": at, "admitted_by": actorID, "no_issue_reason": reason}).Error
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
		return fn(&GORMRepository{db: tx, inTransaction: true})
	})
}
