//go:build integration

package observation

import (
	"citywall/server/internal/infrastructure/recognizer"
	"citywall/server/internal/module/task"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"gorm.io/gorm"
)

func taskObservation(t *testing.T) (*service, *gorm.DB, *Observation, *task.Task) {
	t.Helper()
	s, db, o := persistentObservation(t)
	parent := &task.Task{Title: "任务状态事务验收", AssigneeID: 3, Status: task.StatusAssigned, Revision: 1}
	if err := db.Create(parent).Error; err != nil {
		t.Fatal(err)
	}
	point := &task.TaskPoint{TaskID: parent.ID, PointID: 1, SequenceNo: 1}
	if err := db.Create(point).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Observation{}).Where("id = ?", o.ID).Updates(map[string]any{"source": SourceTask, "task_point_id": point.ID}).Error; err != nil {
		t.Fatal(err)
	}
	o.Source, o.TaskPointID = SourceTask, &point.ID
	s.taskPoints = task.NewGORMRepository(db)
	return s, db, o, parent
}

func TestTaskSubmissionRollsBackWhenTaskStateWriteFails(t *testing.T) {
	s, db, o, parent := taskObservation(t)
	failure := errors.New("task lifecycle storage unavailable")
	callback := "test:fail_task_lifecycle"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "tasks" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Update().Remove(callback) })
	if _, err := s.Submit(context.Background(), inspector(), o.ID, 2); !errors.Is(err, failure) {
		t.Fatalf("err=%v", err)
	}
	saved, err := s.repo.GetByID(context.Background(), o.ID)
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&ObservationSubmission{}).Where("observation_id = ?", o.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(parent, parent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != ObsDraft || saved.Revision != 2 || saved.SubmittedAt != nil || count != 0 || parent.Status != task.StatusAssigned {
		t.Fatalf("partial submission: record=%+v snapshots=%d task=%s", saved, count, parent.Status)
	}
}

func TestCancelledTaskRejectsSubmissionWithoutSnapshot(t *testing.T) {
	s, db, o, parent := taskObservation(t)
	if err := db.Model(parent).Update("status", task.StatusCancelled).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Submit(context.Background(), inspector(), o.ID, 2); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("err=%v", err)
	}
	var count int64
	if err := db.Model(&ObservationSubmission{}).Where("observation_id = ?", o.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	saved, err := s.repo.GetByID(context.Background(), o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 || saved.Revision != 2 || saved.Status != ObsDraft {
		t.Fatal("cancelled task was submitted")
	}
}

func TestConcurrentTaskRecordCreationReturnsDuplicate(t *testing.T) {
	s, db, _, parent := taskObservation(t)
	point := &task.TaskPoint{TaskID: parent.ID, PointID: 2, SequenceNo: 2}
	if err := db.Create(point).Error; err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := s.Create(context.Background(), inspector(), SourceTask, &point.ID, "并发采集验收")
			results <- err
		}()
	}
	close(start)
	success, duplicates := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, ErrDuplicateRecord) {
			duplicates++
		} else {
			t.Errorf("err=%v", err)
		}
	}
	var count int64
	if err := db.Model(&Observation{}).Where("task_point_id = ?", point.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if success != 1 || duplicates != 1 || count != 1 {
		t.Fatalf("success=%d duplicates=%d count=%d", success, duplicates, count)
	}
}

func TestConcurrentRecognitions(t *testing.T) {
	s, db, o := persistentObservation(t)
	s.recognizer = &fakeRecognizer{res: &recognizer.Result{ModelVersion: "test", Detections: []recognizer.Detection{{DetectionID: "d0", ClassCode: "CRACK", Confidence: 0.8, BBox: recognizer.BBox{X1: 0, Y1: 0, X2: 1, Y2: 1}}}}}
	ctx := context.Background()
	links, err := s.repo.ListImages(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errorsFound := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, _, err := s.Recognize(ctx, inspector(), links[0].ImageID, "parallel")
			errorsFound <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-errorsFound; err != nil {
			t.Fatal(err)
		}
	}
	var attempts []RecognitionAttempt
	if err := db.Where("image_id = ?", links[0].ImageID).Order("attempt_no").Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].AttemptNo != 1 || attempts[1].AttemptNo != 2 || attempts[0].Status != AttemptSucceeded || attempts[1].Status != AttemptSucceeded {
		t.Fatalf("attempts=%+v", attempts)
	}
	detections, err := s.repo.ListDetections(ctx, attempts[0].ID)
	if err != nil || len(detections) != 1 {
		t.Fatalf("detections=%+v err=%v", detections, err)
	}
	if belongs, err := s.repo.DetectionBelongsToImage(ctx, detections[0].ID, links[0].ImageID); err != nil || !belongs {
		t.Fatalf("belongs=%v err=%v", belongs, err)
	}
	if belongs, err := s.repo.DetectionBelongsToImage(ctx, detections[0].ID, links[0].ImageID+1000); err != nil || belongs {
		t.Fatalf("belongs=%v err=%v", belongs, err)
	}
}

func TestRecognitionPersistenceRollsBack(t *testing.T) {
	s, db, o := persistentObservation(t)
	ctx := context.Background()
	links, _ := s.repo.ListImages(ctx, o.ID)
	s.recognizer = &fakeRecognizer{res: &recognizer.Result{ModelVersion: "test", Detections: []recognizer.Detection{{DetectionID: "d0", ClassCode: "CRACK", Confidence: 0.8, BBox: recognizer.BBox{X1: 0, Y1: 0, X2: 1, Y2: 1}}}}}
	callback := "test:fail_detection"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "detections" {
			tx.AddError(errors.New("detection storage unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Create().Remove(callback) })
	attempt, _, err := s.Recognize(ctx, inspector(), links[0].ImageID, "rollback")
	if err == nil || attempt.Status != AttemptFailed {
		t.Fatalf("attempt=%+v err=%v", attempt, err)
	}
	var saved RecognitionAttempt
	if err := db.First(&saved, attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&Detection{}).Where("attempt_id = ?", attempt.ID).Count(&count)
	if saved.Status != AttemptFailed || count != 0 {
		t.Fatalf("status=%s detections=%d", saved.Status, count)
	}
}

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
