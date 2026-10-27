package observation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"citywall/server/internal/middleware"
)

type Evidence struct {
	Submissions []SubmissionEvidence `json:"submissions"`
	Observation *Observation         `json:"observation"`
	Attempts    []RecognitionAttempt `json:"attempts"`
	Detections  []Detection          `json:"detections"`
	Conclusions []ConclusionRevision `json:"conclusions"`
	Items       []ConclusionItem     `json:"items"`
	Reviews     []ReviewDecision     `json:"reviews"`
}

type SubmissionLocation struct {
	LocationText string     `json:"location_text"`
	Latitude     *float64   `json:"latitude"`
	Longitude    *float64   `json:"longitude"`
	AccuracyM    *float64   `json:"accuracy_m"`
	DistanceM    *float64   `json:"distance_m"`
	CapturedAt   *time.Time `json:"captured_at"`
}

type SubmissionEvidence struct {
	ID                   int64              `json:"id"`
	SubmissionNo         int                `json:"submission_no"`
	ObservationRevision  int64              `json:"observation_revision"`
	ConclusionRevisionID *int64             `json:"conclusion_revision_id"`
	Location             SubmissionLocation `json:"location"`
	Images               []ImageMeta        `json:"images"`
	SubmittedAt          time.Time          `json:"submitted_at"`
}

func (s *service) Evidence(ctx context.Context, actor *middleware.Actor, id int64) (*Evidence, error) {
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	allowed, err := s.repo.CanReadObservation(ctx, id, actor.UserID, isAdminOrReviewer(actor))
	if err != nil {
		return nil, err
	}
	if !allowed {
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
		Submissions: []SubmissionEvidence{},
		Attempts:    []RecognitionAttempt{}, Detections: []Detection{},
		Conclusions: []ConclusionRevision{}, Items: []ConclusionItem{}, Reviews: []ReviewDecision{},
	}
	var links []ObservationImage
	if err := db.Where("observation_id = ?", id).Find(&links).Error; err != nil {
		return nil, err
	}
	roles, ids := map[int64]string{}, map[int64]bool{}
	for _, link := range links {
		roles[link.ImageID] = link.ImageRole
		ids[link.ImageID] = true
	}
	var submissions []ObservationSubmission
	if err := db.Where("observation_id = ?", id).Order("submission_no, id").Find(&submissions).Error; err != nil {
		return nil, err
	}
	imageIDsBySubmission := map[int64][]int64{}
	for _, submission := range submissions {
		var imageIDs []int64
		if err := json.Unmarshal([]byte(submission.ImageIDsSnapshotJSON), &imageIDs); err != nil {
			return nil, fmt.Errorf("invalid submission images: %w", err)
		}
		for _, imageID := range imageIDs {
			ids[imageID] = true
		}
		imageIDsBySubmission[submission.ID] = imageIDs
	}
	imageIDs := make([]int64, 0, len(ids))
	for imageID := range ids {
		imageIDs = append(imageIDs, imageID)
	}
	var images []Image
	if err := db.Where("id IN ?", imageIDs).Find(&images).Error; err != nil {
		return nil, err
	}
	imageByID := map[int64]Image{}
	for _, image := range images {
		imageByID[image.ID] = image
	}
	for _, submission := range submissions {
		view := SubmissionEvidence{ID: submission.ID, SubmissionNo: submission.SubmissionNo, ObservationRevision: submission.ObservationRevision,
			ConclusionRevisionID: submission.ConclusionRevisionID, SubmittedAt: submission.SubmittedAt, Images: []ImageMeta{}}
		if err := json.Unmarshal([]byte(submission.LocationSnapshotJSON), &view.Location); err != nil {
			return nil, fmt.Errorf("invalid submission location: %w", err)
		}
		for _, imageID := range imageIDsBySubmission[submission.ID] {
			image, ok := imageByID[imageID]
			if !ok {
				return nil, fmt.Errorf("submission image %d is unavailable", imageID)
			}
			role := roles[imageID]
			if role == "" {
				role = "ARCHIVED"
			}
			view.Images = append(view.Images, imageMetadata(&image, role))
		}
		result.Submissions = append(result.Submissions, view)
	}
	if err := db.Where("image_id IN ?", imageIDs).Order("id").Find(&result.Attempts).Error; err != nil {
		return nil, err
	}
	attempts := db.Table("recognition_attempts").Select("id").Where("image_id IN ?", imageIDs)
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
	submissionIDs := make([]int64, 0, len(submissions))
	for _, submission := range submissions {
		submissionIDs = append(submissionIDs, submission.ID)
	}
	var reviews []ReviewDecision
	if err := db.Where("submission_id IN ?", submissionIDs).Find(&reviews).Error; err != nil {
		return nil, err
	}
	reviewBySubmission := map[int64]ReviewDecision{}
	for _, review := range reviews {
		reviewBySubmission[review.SubmissionID] = review
	}
	for _, submission := range submissions {
		if review, ok := reviewBySubmission[submission.ID]; ok {
			result.Reviews = append(result.Reviews, review)
		}
	}
	return result, nil
}
