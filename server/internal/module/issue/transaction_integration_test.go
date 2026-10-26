//go:build integration

package issue

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"citywall/server/internal/infrastructure/database"
	"citywall/server/internal/module/audit"
	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func persistentIssue(t *testing.T, status string) (*service, *gorm.DB, *Issue) {
	t.Helper()
	dsn := os.Getenv("CITY_WALL_TEST_DSN")
	if dsn == "" {
		t.Skip("CITY_WALL_TEST_DSN is required")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil || !strings.HasSuffix(cfg.DBName, "_test") {
		t.Fatal("an isolated test database is required")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := database.Migrate(sqlDB, filepath.Join("..", "..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	assignee := int64(3)
	i := &Issue{Title: "事务验证", Status: status, PriorityCode: PriorityWatch, AssigneeID: &assignee, Revision: 1, OpenedAt: time.Now().UTC()}
	if status == StatusUnassigned {
		i.AssigneeID = nil
	}
	if err := db.Create(i).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Where("issue_id = ?", i.ID).Delete(&IssueAction{}); db.Delete(i) })
	s := NewService(NewGORMRepository(db), nil, nil, audit.NewRecorder(nil)).(*service)
	return s, db, i
}

func TestIssueActionRollsBackOnStatusFailure(t *testing.T) {
	s, db, i := persistentIssue(t, StatusOpen)
	callback := "test:fail_issue_status"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "issues" {
			tx.AddError(errors.New("status write unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Update().Remove(callback) })
	_, err := s.AddAction(context.Background(), inspector(3), i.ID, 1, ActionInput{ActionType: ActionProgress, Content: "处理进展"})
	if err == nil {
		t.Fatal("expected injected failure")
	}
	var saved Issue
	db.First(&saved, i.ID)
	var count int64
	db.Model(&IssueAction{}).Where("issue_id = ?", i.ID).Count(&count)
	if saved.Status != StatusOpen || saved.Revision != 1 || count != 0 {
		t.Fatalf("state=%s revision=%d actions=%d", saved.Status, saved.Revision, count)
	}
}

func TestIssueAssignmentRollsBackOnActionFailure(t *testing.T) {
	s, db, i := persistentIssue(t, StatusUnassigned)
	callback := "test:fail_issue_action"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "issue_actions" {
			tx.AddError(errors.New("action write unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Create().Remove(callback) })
	_, err := s.Assign(context.Background(), admin(), i.ID, 1, 3, "指派")
	if err == nil {
		t.Fatal("expected injected failure")
	}
	var saved Issue
	db.First(&saved, i.ID)
	if saved.Status != StatusUnassigned || saved.Revision != 1 || saved.AssigneeID != nil {
		t.Fatalf("partial assignment: %+v", saved)
	}
}

func TestConcurrentIssueActionsHaveOneWinner(t *testing.T) {
	s, db, i := persistentIssue(t, StatusOpen)
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := s.AddAction(context.Background(), inspector(3), i.ID, 1, ActionInput{ActionType: ActionProgress, Content: "处理进展"})
			results <- err
		}()
	}
	close(start)
	wins, conflicts := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			wins++
		} else if errors.Is(err, ErrStatusConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	var count int64
	db.Model(&IssueAction{}).Where("issue_id = ?", i.ID).Count(&count)
	if wins != 1 || conflicts != 1 || count != 1 {
		t.Fatalf("wins=%d conflicts=%d actions=%d", wins, conflicts, count)
	}
}

func TestIssueTransitionsRollBackOnStatusFailure(t *testing.T) {
	cases := []struct {
		name, status string
		apply        func(*service, int64) error
	}{
		{"request", StatusInProgress, func(s *service, id int64) error {
			_, err := s.RequestVerification(context.Background(), inspector(3), id, 1, "请核验")
			return err
		}},
		{"verify", StatusPendingVerification, func(s *service, id int64) error {
			_, err := s.Verify(context.Background(), reviewer(), id, 1, "PASS", "确认关闭")
			return err
		}},
		{"reopen", StatusClosed, func(s *service, id int64) error {
			_, err := s.Reopen(context.Background(), reviewer(), id, 1, "复查")
			return err
		}},
		{"assign", StatusUnassigned, func(s *service, id int64) error {
			_, err := s.Assign(context.Background(), admin(), id, 1, 3, "指派")
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, db, i := persistentIssue(t, tc.status)
			if tc.status == StatusClosed {
				now := time.Now().UTC()
				if err := db.Model(i).Updates(map[string]any{"closed_at": now, "close_reason": "原关闭原因"}).Error; err != nil {
					t.Fatal(err)
				}
			}
			callback := "test:fail_transition"
			if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
				if tx.Statement.Table != "issues" {
					return
				}
				updates, ok := tx.Statement.Dest.(map[string]interface{})
				if ok && updates["status"] != nil {
					tx.AddError(errors.New("status unavailable"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Callback().Update().Remove(callback) })
			if err := tc.apply(s, i.ID); err == nil {
				t.Fatal("expected injected failure")
			}
			var saved Issue
			if err := db.First(&saved, i.ID).Error; err != nil {
				t.Fatal(err)
			}
			var count int64
			db.Model(&IssueAction{}).Where("issue_id = ?", i.ID).Count(&count)
			if saved.Status != tc.status || saved.Revision != 1 || count != 0 {
				t.Fatalf("partial transition: %+v actions=%d", saved, count)
			}
			if tc.status == StatusUnassigned && saved.AssigneeID != nil {
				t.Fatal("assignment not rolled back")
			}
			if tc.status == StatusClosed && (saved.ClosedAt == nil || saved.CloseReason != "原关闭原因") {
				t.Fatal("closure not preserved")
			}
			if tc.status == StatusPendingVerification && (saved.ClosedAt != nil || saved.CloseReason != "") {
				t.Fatal("partial closure persisted")
			}
		})
	}
}

func TestConcurrentIssueVerificationHasOneWinner(t *testing.T) {
	s, db, i := persistentIssue(t, StatusPendingVerification)
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := s.Verify(context.Background(), reviewer(), i.ID, 1, "PASS", "核验通过")
			results <- err
		}()
	}
	close(start)
	wins := 0
	for range 2 {
		err := <-results
		if err == nil {
			wins++
		} else if !errors.Is(err, ErrStatusConflict) && !errors.Is(err, ErrInvalidState) {
			t.Fatal(err)
		}
	}
	var saved Issue
	if err := db.First(&saved, i.ID).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&IssueAction{}).Where("issue_id = ?", i.ID).Count(&count)
	if wins != 1 || count != 1 || saved.Status != StatusClosed || saved.Revision != 2 || saved.ClosedAt == nil || saved.CloseReason != "核验通过" {
		t.Fatalf("wins=%d actions=%d issue=%+v", wins, count, saved)
	}
}

type abortingIssueRepository struct{ Repository }

func (r abortingIssueRepository) InTx(ctx context.Context, fn func(Repository) error) error {
	return r.Repository.InTx(ctx, func(tx Repository) error {
		if err := fn(tx); err != nil {
			return err
		}
		return errors.New("transaction aborted before commit")
	})
}

func TestIssueAuditFollowsCommittedState(t *testing.T) {
	s, db, i := persistentIssue(t, StatusOpen)
	recorder := &issueAuditBuffer{}
	s.audit = recorder
	original := s.repo
	s.repo = abortingIssueRepository{original}
	_, err := s.AddAction(context.Background(), inspector(3), i.ID, 1, ActionInput{ActionType: ActionProgress, Content: "处理进展"})
	if err == nil || len(recorder.events) != 0 {
		t.Fatalf("err=%v audit events=%d", err, len(recorder.events))
	}
	var saved Issue
	if err := db.First(&saved, i.ID).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&IssueAction{}).Where("issue_id = ?", i.ID).Count(&count)
	if saved.Status != StatusOpen || saved.Revision != 1 || count != 0 {
		t.Fatalf("partial transaction: %+v actions=%d", saved, count)
	}
	s.repo = original
	if _, err := s.AddAction(context.Background(), inspector(3), i.ID, 1, ActionInput{ActionType: ActionProgress, Content: "处理进展"}); err != nil {
		t.Fatal(err)
	}
	if len(recorder.events) != 1 || recorder.events[0].OldStatus != StatusOpen || recorder.events[0].NewStatus != StatusInProgress {
		t.Fatalf("audit events=%+v", recorder.events)
	}
}
