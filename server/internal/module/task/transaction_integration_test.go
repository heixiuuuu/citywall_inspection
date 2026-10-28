//go:build integration

package task

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"citywall/server/internal/module/audit"
	"gorm.io/gorm"
)

func persistentTaskService(t *testing.T) (Service, *GORMRepository, int64) {
	t.Helper()
	repo := newTxRepo(t, integrationDB(t))
	user := struct {
		ID          int64
		DisplayName string
		Status      string
	}{DisplayName: "任务事务验收", Status: "ACTIVE"}
	if err := repo.db.Table("users").Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Exec("INSERT INTO user_roles (user_id, role_code) VALUES (?, 'INSPECTOR')", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewService(repo, &fakeObsStatus{}, func(_ context.Context, id int64) (PointSnapshot, error) {
		return PointSnapshot{PointID: id, Name: "派发点位", SegmentID: 1, Latitude: 1, Longitude: 2, RadiusM: 100}, nil
	}, audit.NewRecorder(nil))
	return svc, repo, user.ID
}

func TestTaskCreateRollsBackWhenPointsFail(t *testing.T) {
	svc, repo, assignee := persistentTaskService(t)
	failure := errors.New("task point storage unavailable")
	callback := "test:fail_task_points"
	if err := repo.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "task_points" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.db.Callback().Create().Remove(callback) })
	_, err := svc.Create(context.Background(), admin(), CreateInput{Title: t.Name(), AssigneeID: assignee, PointIDs: []int64{1}})
	if !errors.Is(err, failure) {
		t.Fatalf("create error: %v", err)
	}
	var count int64
	if err := repo.db.Model(&Task{}).Where("title = ?", t.Name()).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed task creation left an orphan task")
	}
}

func TestTaskDispatchRollsBackSnapshotsWhenStatusFails(t *testing.T) {
	svc, repo, assignee := persistentTaskService(t)
	ctx := context.Background()
	task, err := svc.Create(ctx, admin(), CreateInput{Title: t.Name(), AssigneeID: assignee, PointIDs: []int64{1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("task status storage unavailable")
	callback := "test:fail_task_status"
	if err := repo.db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "tasks" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.db.Callback().Update().Remove(callback) })
	if _, err := svc.Dispatch(ctx, admin(), task.ID, 1); !errors.Is(err, failure) {
		t.Fatalf("dispatch error: %v", err)
	}
	saved, err := repo.GetByID(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	points, err := repo.ListPoints(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != StatusDraft || saved.Revision != 1 || saved.DispatchedAt != nil {
		t.Fatalf("failed dispatch changed task: %+v", saved)
	}
	for _, point := range points {
		if point.SnapshotName != nil || point.SnapshotLatitude != nil || point.SnapshotRadiusM != nil {
			t.Fatal("failed dispatch left point snapshots")
		}
	}
}

func TestConcurrentTaskDispatchPreservesWinningSnapshot(t *testing.T) {
	db := integrationDB(t)
	user := struct {
		ID          int64
		DisplayName string
		Status      string
	}{DisplayName: "并发派发验收", Status: "ACTIVE"}
	if err := db.Table("users").Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	var taskID int64
	t.Cleanup(func() {
		db.Where("task_id = ?", taskID).Delete(&TaskPoint{})
		db.Where("id = ?", taskID).Delete(&Task{})
		db.Exec("DELETE FROM user_roles WHERE user_id = ?", user.ID)
		db.Exec("DELETE FROM users WHERE id = ?", user.ID)
	})
	if err := db.Exec("INSERT INTO user_roles (user_id, role_code) VALUES (?, 'INSPECTOR')", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	type snapshotKey struct{}
	svc := NewService(NewGORMRepository(db), &fakeObsStatus{}, func(ctx context.Context, id int64) (PointSnapshot, error) {
		name, _ := ctx.Value(snapshotKey{}).(string)
		return PointSnapshot{PointID: id, Name: name, SegmentID: 1, Latitude: 1, Longitude: 2, RadiusM: 100}, nil
	}, audit.NewRecorder(nil))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	created, err := svc.Create(ctx, admin(), CreateInput{Title: t.Name(), AssigneeID: user.ID, PointIDs: []int64{1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	taskID = created.ID
	type result struct {
		name string
		err  error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for index := range 2 {
		go func() {
			name := "dispatch-" + strconv.Itoa(index)
			<-start
			_, err := svc.Dispatch(context.WithValue(ctx, snapshotKey{}, name), admin(), taskID, 1)
			results <- result{name: name, err: err}
		}()
	}
	close(start)
	winner := ""
	var failures []error
	for range 2 {
		outcome := <-results
		if outcome.err == nil {
			if winner != "" {
				t.Fatal("both dispatches succeeded at the same revision")
			}
			winner = outcome.name
		} else {
			failures = append(failures, outcome.err)
		}
	}
	if winner == "" || len(failures) != 1 || !errors.Is(failures[0], ErrInvalidState) {
		t.Fatalf("winner=%q failures=%v", winner, failures)
	}
	saved, points, err := svc.Get(ctx, admin(), taskID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != StatusAssigned || saved.Revision != 2 || saved.DispatchedAt == nil || len(points) != 2 {
		t.Fatalf("unexpected dispatched task: %+v points=%d", saved, len(points))
	}
	for _, point := range points {
		if point.SnapshotName == nil || *point.SnapshotName != winner {
			t.Fatal("failed concurrent dispatch overwrote winning snapshot")
		}
	}
}
