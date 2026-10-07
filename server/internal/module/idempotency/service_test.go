package idempotency

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	gdb, err := gorm.Open(mysql.New(mysql.Config{Conn: db, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	return gdb, mock
}

func TestCheckThenRecord(t *testing.T) {
	gdb, mock := newMockDB(t)
	svc := NewService(NewRepository(gdb))
	ctx := context.Background()

	// 首次：查无记录 → (false,nil,nil)，不写。
	mock.ExpectQuery("SELECT .* FROM `idempotency_records`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "actor_id", "operation", "idem_key", "request_hash", "response_ref", "status", "expires_at", "created_at"}))
	done, rec, err := svc.Check(ctx, 3, "POST /tasks/1/dispatch", "key-a")
	if err != nil {
		t.Fatalf("first check: %v", err)
	}
	if done || rec != nil {
		t.Fatalf("first check should not be replay: done=%v rec=%v", done, rec)
	}

	// 成功后补写记录。
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `idempotency_records`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := svc.Record(ctx, 3, "POST /tasks/1/dispatch", "key-a", "hash1"); err != nil {
		t.Fatalf("record: %v", err)
	}

	// 重放：查到 COMPLETED → (true,rec,nil)
	mock.ExpectQuery("SELECT .* FROM `idempotency_records`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "actor_id", "operation", "idem_key", "request_hash", "response_ref", "status", "expires_at", "created_at"}).
			AddRow(1, 3, "POST /tasks/1/dispatch", "key-a", "hash1", "", "COMPLETED", nil, nil))
	done, rec, err = svc.Check(ctx, 3, "POST /tasks/1/dispatch", "key-a")
	if err != nil {
		t.Fatalf("replay check: %v", err)
	}
	if !done || rec == nil || rec.ID != 1 {
		t.Fatalf("replay should return existing record: done=%v rec=%v", done, rec)
	}
}

func TestConcurrentRecordConflictIgnored(t *testing.T) {
	gdb, mock := newMockDB(t)
	svc := NewService(NewRepository(gdb))
	ctx := context.Background()

	// 并发双写：唯一键冲突 → 忽略，不报错。
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `idempotency_records`").
		WillReturnError(gorm.ErrDuplicatedKey)
	mock.ExpectRollback()
	if err := svc.Record(ctx, 3, "POST /tasks/1/dispatch", "key-a", "hash1"); err != nil {
		t.Fatalf("record conflict should be ignored: %v", err)
	}
}

func TestFailedStatusAllowsRetry(t *testing.T) {
	gdb, mock := newMockDB(t)
	svc := NewService(NewRepository(gdb))
	ctx := context.Background()

	// CONFLICT/失败态记录 → 允许重试（false）
	mock.ExpectQuery("SELECT .* FROM `idempotency_records`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "actor_id", "operation", "idem_key", "request_hash", "response_ref", "status", "expires_at", "created_at"}).
			AddRow(2, 3, "POST /tasks/1/dispatch", "key-b", "hash9", "", "CONFLICT", nil, nil))
	done, rec, err := svc.Check(ctx, 3, "POST /tasks/1/dispatch", "key-b")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if done || rec != nil {
		t.Fatalf("failed record must allow retry: done=%v rec=%v", done, rec)
	}
}
