package observation

import (
	"context"

	"gorm.io/gorm"
)

// Repository 现场记录数据访问接口。
type Repository interface {
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

	ListByStatuses(ctx context.Context, statuses []string) ([]Observation, error)
	ListPage(ctx context.Context, reporterID int64, isAdmin bool, status string, limit, offset int) ([]Observation, error)
	CountObservations(ctx context.Context, reporterID int64, isAdmin bool, status string) (int64, error)
	StatusByTaskPoints(ctx context.Context, taskPointIDs []int64) (map[int64]string, error)
	ObservationByImage(ctx context.Context, imageID int64) (*Observation, error)
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
	res := r.db.WithContext(ctx).
		Model(&Observation{}).
		Where("id = ? AND revision = ?", id, fromRevision).
		Updates(map[string]any{"status": status, "revision": gorm.Expr("revision + 1")})
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

func (r *GORMRepository) ListByStatuses(ctx context.Context, statuses []string) ([]Observation, error) {
	var obs []Observation
	return obs, r.db.WithContext(ctx).
		Where("status IN ?", statuses).
		Order("updated_at DESC").
		Find(&obs).Error
}

// ListPage 分页 + 状态过滤查询（管理员/审核员看全部，巡检员只看自己的上报）。
func (r *GORMRepository) ListPage(ctx context.Context, reporterID int64, isAdmin bool, status string, limit, offset int) ([]Observation, error) {
	var obs []Observation
	q := r.db.WithContext(ctx).Order("updated_at DESC")
	if !isAdmin {
		q = q.Where("reporter_id = ?", reporterID)
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
	return obs, q.Find(&obs).Error
}

// CountObservations 统计记录数（与 ListPage 同一过滤条件）。
func (r *GORMRepository) CountObservations(ctx context.Context, reporterID int64, isAdmin bool, status string) (int64, error) {
	var total int64
	q := r.db.WithContext(ctx).Model(&Observation{})
	if !isAdmin {
		q = q.Where("reporter_id = ?", reporterID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	return total, q.Count(&total).Error
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
