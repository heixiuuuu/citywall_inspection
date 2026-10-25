//go:build integration

package observation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"gorm.io/gorm"
)

func persistentObservation(t *testing.T) (*service, *gorm.DB, *Observation) {
	t.Helper()
	db := integrationDB(t)
	s, _ := newObsService()
	s.repo = NewGORMRepository(db)
	zero := float64(0)
	o := &Observation{Source: SourceVoluntary, ReporterID: 3, LocationText: "事务验收", Latitude: &zero, Longitude: &zero, Status: ObsDraft, Revision: 1}
	if err := s.repo.Insert(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	image := &Image{StorageKey: fmt.Sprintf("transaction/%d.png", o.ID), SHA256: "test", MediaType: "image/png", ByteSize: 10, Width: 1, Height: 1, UploadedBy: 3}
	if err := s.repo.InsertImage(context.Background(), image); err != nil {
		t.Fatal(err)
	}
	if err := s.repo.LinkImage(context.Background(), o.ID, image.ID, ImageRoleOverview); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveConclusion(context.Background(), inspector(), o.ID, 1, ConclusionInput{Outcome: OutcomeNormal}); err != nil {
		t.Fatal(err)
	}
	return s, db, o
}

func concurrentOperations(t *testing.T, operations ...func() error) {
	t.Helper()
	start := make(chan struct{})
	outcomes := make(chan error, len(operations))
	var group sync.WaitGroup
	for _, operation := range operations {
		group.Add(1)
		go func(operation func() error) { defer group.Done(); <-start; outcomes <- operation() }(operation)
	}
	close(start)
	group.Wait()
	close(outcomes)
	succeeded, conflicted := 0, 0
	for err := range outcomes {
		if err == nil {
			succeeded++
		} else if errors.Is(err, ErrStatusConflict) || errors.Is(err, ErrInvalidState) {
			conflicted++
		} else {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || conflicted != len(operations)-1 {
		t.Fatalf("success=%d conflicts=%d", succeeded, conflicted)
	}
}

func TestConcurrentSubmissionAndReview(t *testing.T) {
	s, db, o := persistentObservation(t)
	ctx := context.Background()
	submit := func() error { _, err := s.Submit(ctx, inspector(), o.ID, 2); return err }
	concurrentOperations(t, submit, submit)
	var submissions []ObservationSubmission
	if err := db.Where("observation_id = ?", o.ID).Find(&submissions).Error; err != nil {
		t.Fatal(err)
	}
	if len(submissions) != 1 {
		t.Fatalf("submissions=%d", len(submissions))
	}
	concurrentOperations(t,
		func() error {
			_, err := s.Review(ctx, admin(), o.ID, 3, ReviewInput{Decision: ReviewAccept})
			return err
		},
		func() error {
			_, err := s.Review(ctx, reviewer(), o.ID, 3, ReviewInput{Decision: ReviewReturn, Reason: "补充近景"})
			return err
		},
	)
	var decisions []ReviewDecision
	if err := db.Where("submission_id = ?", submissions[0].ID).Find(&decisions).Error; err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 {
		t.Fatalf("decisions=%d", len(decisions))
	}
	current, err := s.repo.GetByID(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	expected := ObsAccepted
	if decisions[0].Decision == ReviewReturn {
		expected = ObsReturned
	}
	if current.Status != expected || current.Revision != 4 || current.SubmittedAt == nil || current.ReviewedAt == nil {
		t.Fatalf("inconsistent observation: %+v", current)
	}
}

func TestSubmissionFailureRollsBackSnapshot(t *testing.T) {
	s, db, o := persistentObservation(t)
	injected := errors.New("injected status write failure")
	const callback = "test:fail_submission_status"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		changes, ok := tx.Statement.Dest.(map[string]any)
		if tx.Statement.Table == "observations" && ok && changes["status"] == ObsSubmitted {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Update().Remove(callback) })
	if _, err := s.Submit(context.Background(), inspector(), o.ID, 2); !errors.Is(err, injected) {
		t.Fatalf("got %v", err)
	}
	var count int64
	if err := db.Model(&ObservationSubmission{}).Where("observation_id = ?", o.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("snapshot survived rollback: %d", count)
	}
	current, err := s.repo.GetByID(context.Background(), o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != ObsDraft || current.Revision != 2 {
		t.Fatalf("unexpected state: %+v", current)
	}
}
