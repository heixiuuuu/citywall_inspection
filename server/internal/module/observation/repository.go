package observation

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Repository 现场记录数据访问接口。
type Repository interface {
	UnlinkImage(ctx context.Context, observationID, imageID int64) error
	LoadEvidence(ctx context.Context, id int64) (*Evidence, error)
	Insert(ctx context.Context, o *Observation) error
	GetByID(ctx context.Context, id int64) (*Observation, error)
	GetByTaskPoint(ctx context.Context, taskPointID int64) (*Observation, error)
	Update(ctx context.Context, o *Observation) error
	BumpRevision(ctx context.Context, id int64, fromRevision int64) (int64, error)
	UpdateStatus(ctx context.Context, id int64, fromRevision int64, status string) (int64, error)

	InsertImage(ctx context.Context, img *Image) error
	GetImage(ctx context.Context, id int64) (*Image, error)
	LinkImage(ctx context.Context, obsID, imageID int64, role string) error
	ListImages(ctx context.Context, obsID int64) ([]ObservationImage, error)

	NextAttemptNo(ctx context.Context, imageID int64) (int, error)
	InsertAttempt(ctx context.Context, a *RecognitionAttempt) error
	UpdateAttempt(ctx context.Context, a *RecognitionAttempt) error
	InsertDetections(ctx context.Context, ds []Detection) error
	ListDetections(ctx context.Context, attemptID int64) ([]Detection, error)

	NextConclusionNo(ctx context.Context, obsID int64) (int, error)
	InsertConclusion(ctx context.Context, rev *ConclusionRevision, items []ConclusionItem) error
	LatestConclusion(ctx context.Context, obsID int64) (*ConclusionRevision, error)

	NextSubmissionNo(ctx context.Context, obsID int64) (int, error)
	InsertSubmission(ctx context.Context, s *ObservationSubmission) error
	LatestSubmission(ctx context.Context, obsID int64) (*ObservationSubmission, error)
	InsertReviewDecision(ctx context.Context, d *ReviewDecision) error
	LatestReviewDecision(ctx context.Context, obsID int64) (*ReviewDecision, error)

	ListByStatuses(ctx context.Context, statuses []string) ([]Observation, error)
	ListPage(ctx context.Context, q ListQuery) ([]Observation, error)
	CountObservations(ctx context.Context, q ListQuery) (int64, error)
	StatusByTaskPoints(ctx context.Context, taskPointIDs []int64) (map[int64]string, error)
	ObservationByImage(ctx context.Context, imageID int64) (*Observation, error)
}

func (r *GORMRepository) UnlinkImage(ctx context.Context, observationID, imageID int64) error {
	return r.db.WithContext(ctx).Where("observation_id = ? AND image_id = ?", observationID, imageID).Delete(&ObservationImage{}).Error
}

// GORMRepository 基于 GORM 的实现。
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository 构造现场记录仓储。
func NewGORMRepository(db *gorm.DB) Repository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) Insert(ctx context.Context, o *Observation) error {
	return r.db.WithContext(ctx).Create(o).Error
}

func (r *GORMRepository) GetByID(ctx context.Context, id int64) (*Observation, error) {
	var o Observation
	if err := r.db.WithContext(ctx).First(&o, id).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *GORMRepository) GetByTaskPoint(ctx context.Context, taskPointID int64) (*Observation, error) {
	var o Observation
	if err := r.db.WithContext(ctx).Where("task_point_id = ?", taskPointID).First(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *GORMRepository) Update(ctx context.Context, o *Observation) error {
	return r.db.WithContext(ctx).Save(o).Error
}

func (r *GORMRepository) BumpRevision(ctx context.Context, id int64, fromRevision int64) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&Observation{}).
		Where("id = ? AND revision = ?", id, fromRevision).
		Update("revision", gorm.Expr("revision + 1"))
	return res.RowsAffected, res.Error
}

func (r *GORMRepository) UpdateStatus(ctx context.Context, id int64, fromRevision int64, status string) (int64, error) {
	changes := map[string]any{"status": status, "revision": gorm.Expr("revision + 1")}
	if status == ObsSubmitted {
		changes["submitted_at"] = time.Now().UTC()
		changes["reviewed_at"] = nil
		changes["inspection_outcome"] = gorm.Expr("(SELECT outcome FROM conclusion_revisions WHERE observation_id = ? ORDER BY revision_no DESC LIMIT 1)", id)
	}
	if status == ObsAccepted || status == ObsReturned || status == ObsRejected {
		changes["reviewed_at"] = time.Now().UTC()
	}
	res := r.db.WithContext(ctx).
		Model(&Observation{}).
		Where("id = ? AND revision = ?", id, fromRevision).
		Updates(changes)
	return res.RowsAffected, res.Error
}

func (r *GORMRepository) InsertImage(ctx context.Context, img *Image) error {
	return r.db.WithContext(ctx).Create(img).Error
}

func (r *GORMRepository) GetImage(ctx context.Context, id int64) (*Image, error) {
	var img Image
	if err := r.db.WithContext(ctx).First(&img, id).Error; err != nil {
		return nil, err
	}
	return &img, nil
}

func (r *GORMRepository) LinkImage(ctx context.Context, obsID, imageID int64, role string) error {
	return r.db.WithContext(ctx).Create(&ObservationImage{
		ObservationID: obsID,
		ImageID:       imageID,
		ImageRole:     role,
	}).Error
}

func (r *GORMRepository) ListImages(ctx context.Context, obsID int64) ([]ObservationImage, error) {
	var links []ObservationImage
	return links, r.db.WithContext(ctx).
		Where("observation_id = ?", obsID).
		Find(&links).Error
}

func (r *GORMRepository) NextAttemptNo(ctx context.Context, imageID int64) (int, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&RecognitionAttempt{}).
		Where("image_id = ?", imageID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count) + 1, nil
}

func (r *GORMRepository) InsertAttempt(ctx context.Context, a *RecognitionAttempt) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *GORMRepository) UpdateAttempt(ctx context.Context, a *RecognitionAttempt) error {
	return r.db.WithContext(ctx).Save(a).Error
}

func (r *GORMRepository) InsertDetections(ctx context.Context, ds []Detection) error {
	if len(ds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&ds).Error
}

func (r *GORMRepository) ListDetections(ctx context.Context, attemptID int64) ([]Detection, error) {
	var ds []Detection
	return ds, r.db.WithContext(ctx).
		Where("attempt_id = ?", attemptID).
		Order("id").Find(&ds).Error
}

func (r *GORMRepository) NextConclusionNo(ctx context.Context, obsID int64) (int, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&ConclusionRevision{}).
		Where("observation_id = ?", obsID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count) + 1, nil
}

func (r *GORMRepository) InsertConclusion(ctx context.Context, rev *ConclusionRevision, items []ConclusionItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(rev).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			for i := range items {
				items[i].ConclusionRevisionID = rev.ID
			}
			return tx.Create(&items).Error
		}
		return nil
	})
}

func (r *GORMRepository) LatestConclusion(ctx context.Context, obsID int64) (*ConclusionRevision, error) {
	var rev ConclusionRevision
	if err := r.db.WithContext(ctx).
		Where("observation_id = ?", obsID).
		Order("revision_no DESC").
		First(&rev).Error; err != nil {
		return nil, err
	}
	return &rev, nil
}

func (r *GORMRepository) NextSubmissionNo(ctx context.Context, obsID int64) (int, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&ObservationSubmission{}).
		Where("observation_id = ?", obsID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count) + 1, nil
}

func (r *GORMRepository) InsertSubmission(ctx context.Context, s *ObservationSubmission) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *GORMRepository) LatestSubmission(ctx context.Context, obsID int64) (*ObservationSubmission, error) {
	var s ObservationSubmission
	if err := r.db.WithContext(ctx).
		Where("observation_id = ?", obsID).
		Order("submission_no DESC").
		First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *GORMRepository) InsertReviewDecision(ctx context.Context, d *ReviewDecision) error {
	return r.db.WithContext(ctx).Create(d).Error
}

// LatestReviewDecision 返回记录最近一次提交的审核决定。
func (r *GORMRepository) LatestReviewDecision(ctx context.Context, obsID int64) (*ReviewDecision, error) {
	var sub ObservationSubmission
	if err := r.db.WithContext(ctx).
		Where("observation_id = ?", obsID).
		Order("submission_no DESC").
		First(&sub).Error; err != nil {
		return nil, err
	}
	var d ReviewDecision
	if err := r.db.WithContext(ctx).Where("submission_id = ?", sub.ID).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *GORMRepository) ListByStatuses(ctx context.Context, statuses []string) ([]Observation, error) {
	var obs []Observation
	return obs, r.db.WithContext(ctx).
		Where("status IN ?", statuses).
		Order("updated_at DESC").
		Find(&obs).Error
}

// ListQuery 记录列表过滤条件（按任务/点位/来源/时间/审核状态）。
type ListQuery struct {
	ReporterID int64
	IsAdmin    bool
	Status     string
	TaskID     int64
	Source     string
	PointID    int64
	DateFrom   *time.Time
	DateTo     *time.Time
	Limit      int
	Offset     int
}

// ListPage 分页查询（管理员/审核员看全部，巡检员只看自己的上报）。
func (r *GORMRepository) ListPage(ctx context.Context, q ListQuery) ([]Observation, error) {
	var obs []Observation
	base := r.db.WithContext(ctx).Order("updated_at DESC")
	base = applyObservationFilter(base, q)
	if q.Limit > 0 {
		base = base.Limit(q.Limit)
		if q.Offset > 0 {
			base = base.Offset(q.Offset)
		}
	}
	return obs, base.Find(&obs).Error
}

// CountObservations 统计记录数（与 ListPage 同一过滤条件）。
func (r *GORMRepository) CountObservations(ctx context.Context, q ListQuery) (int64, error) {
	var total int64
	base := applyObservationFilter(r.db.WithContext(ctx).Model(&Observation{}), q)
	return total, base.Count(&total).Error
}

// applyObservationFilter 组装过滤条件。
func applyObservationFilter(db *gorm.DB, q ListQuery) *gorm.DB {
	if !q.IsAdmin {
		db = db.Where("reporter_id = ?", q.ReporterID)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	if q.Source != "" {
		db = db.Where("source = ?", q.Source)
	}
	if q.PointID > 0 {
		db = db.Where("point_id = ?", q.PointID)
	}
	if q.TaskID > 0 {
		// 任务过滤：observations.task_point_id → task_points.id，且 task_points.task_id = ?
		db = db.Joins("JOIN task_points ON task_points.id = observations.task_point_id").
			Where("task_points.task_id = ?", q.TaskID)
	}
	if q.DateFrom != nil {
		db = db.Where("created_at >= ?", *q.DateFrom)
	}
	if q.DateTo != nil {
		db = db.Where("created_at <= ?", *q.DateTo)
	}
	return db
}

func (r *GORMRepository) StatusByTaskPoints(ctx context.Context, taskPointIDs []int64) (map[int64]string, error) {
	m := map[int64]string{}
	if len(taskPointIDs) == 0 {
		return m, nil
	}
	var obs []Observation
	err := r.db.WithContext(ctx).
		Where("task_point_id IN ?", taskPointIDs).
		Find(&obs).Error
	if err != nil {
		return nil, err
	}
	for _, o := range obs {
		if o.TaskPointID != nil {
			m[*o.TaskPointID] = o.Status
		}
	}
	return m, nil
}

func (r *GORMRepository) ObservationByImage(ctx context.Context, imageID int64) (*Observation, error) {
	var link ObservationImage
	if err := r.db.WithContext(ctx).Where("image_id = ?", imageID).First(&link).Error; err != nil {
		return nil, err
	}
	var o Observation
	if err := r.db.WithContext(ctx).First(&o, link.ObservationID).Error; err != nil {
		return nil, err
	}
	return &o, nil
}
