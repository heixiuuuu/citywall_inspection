package observation

import (
	"context"

	"citywall/server/internal/middleware"
)

type Evidence struct {
	Observation *Observation         `json:"observation"`
	Attempts    []RecognitionAttempt `json:"attempts"`
	Detections  []Detection          `json:"detections"`
	Conclusions []ConclusionRevision `json:"conclusions"`
	Items       []ConclusionItem     `json:"items"`
	Reviews     []ReviewDecision     `json:"reviews"`
}

func (s *service) Evidence(ctx context.Context, actor *middleware.Actor, id int64) (*Evidence, error) {
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if o.ReporterID != actor.UserID && !isAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	evidence, err := s.repo.LoadEvidence(ctx, id)
	if err != nil {
		return nil, err
	}
	evidence.Observation = o
	return evidence, nil
}

func (r *GORMRepository) LoadEvidence(ctx context.Context, id int64) (*Evidence, error) {
	db := r.db.WithContext(ctx)
	result := &Evidence{
		Attempts: []RecognitionAttempt{}, Detections: []Detection{},
		Conclusions: []ConclusionRevision{}, Items: []ConclusionItem{}, Reviews: []ReviewDecision{},
	}
	images := db.Table("observation_images").Select("image_id").Where("observation_id = ?", id)
	if err := db.Where("image_id IN (?)", images).Order("id").Find(&result.Attempts).Error; err != nil {
		return nil, err
	}
	attempts := db.Table("recognition_attempts").Select("id").Where("image_id IN (?)", images)
	if err := db.Where("attempt_id IN (?)", attempts).Order("id").Find(&result.Detections).Error; err != nil {
		return nil, err
	}
	if err := db.Where("observation_id = ?", id).Order("revision_no").Find(&result.Conclusions).Error; err != nil {
		return nil, err
	}
	conclusions := db.Table("conclusion_revisions").Select("id").Where("observation_id = ?", id)
	if err := db.Where("conclusion_revision_id IN (?)", conclusions).Order("id").Find(&result.Items).Error; err != nil {
		return nil, err
	}
	submissions := db.Table("observation_submissions").Select("id").Where("observation_id = ?", id)
	if err := db.Where("submission_id IN (?)", submissions).Order("id").Find(&result.Reviews).Error; err != nil {
		return nil, err
	}
	return result, nil
}
