//go:build integration

// CITY_WALL_TEST_DSN 指定测试库，迁移自动执行，各用例使用回滚事务隔离数据。
package task

import (
	"citywall/server/internal/infrastructure/database"
	"context"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestMain(m *testing.M) {
	db := integrationDB(nil)
	if err := runMigrations(db); err != nil {
		panic("setup migrations: " + err.Error())
	}
	code := m.Run()
	os.Exit(code)
}

// runMigrations 自动应用尚未执行的 SQL 迁移。
func runMigrations(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return database.Migrate(sqlDB, filepath.Join("..", "..", "..", "migrations"))
}

func integrationDB(t *testing.T) *gorm.DB {
	dsn := os.Getenv("CITY_WALL_TEST_DSN")
	if dsn == "" {
		dsn = "city_wall:city_wall_dev@tcp(127.0.0.1:3306)/city_wall_test?charset=utf8mb4&parseTime=True&loc=UTC&multiStatements=true"
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Warn)})
	if err != nil {
		if t != nil {
			t.Fatalf("connect test db: %v", err)
		}
		panic("connect test db: " + err.Error())
	}
	return db
}

// newTxRepo 在事务内构造仓储，测试结束回滚。
func newTxRepo(t *testing.T, db *gorm.DB) *GORMRepository {
	t.Helper()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin tx: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return NewGORMRepository(tx)
}

func TestGORMInsertAndGetTask(t *testing.T) {
	db := integrationDB(t)
	repo := newTxRepo(t, db)
	ctx := context.Background()

	tk := &Task{Title: "it-task", AssigneeID: 3, Status: StatusDraft, Revision: 1}
	if err := repo.InsertTask(ctx, tk); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if tk.ID == 0 {
		t.Fatal("expected auto id")
	}
	got, err := repo.GetByID(ctx, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "it-task" || got.Status != StatusDraft || got.Revision != 1 {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestGORMInsertTaskPointsAndList(t *testing.T) {
	db := integrationDB(t)
	repo := newTxRepo(t, db)
	ctx := context.Background()

	tk := &Task{Title: "it-points", AssigneeID: 3, Status: StatusDraft, Revision: 1}
	if err := repo.InsertTask(ctx, tk); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if err := repo.InsertTaskPoints(ctx, tk.ID, []int64{1, 2, 3}); err != nil {
		t.Fatalf("insert points: %v", err)
	}
	pts, err := repo.ListPoints(ctx, tk.ID)
	if err != nil {
		t.Fatalf("list points: %v", err)
	}
	if len(pts) != 3 {
		t.Fatalf("points len = %d", len(pts))
	}
	for i, p := range pts {
		if p.SequenceNo != i+1 {
			t.Fatalf("seq %d != %d", p.SequenceNo, i+1)
		}
	}
}

func TestGORMOptimisticLockConflict(t *testing.T) {
	db := integrationDB(t)
	repo := newTxRepo(t, db)
	ctx := context.Background()

	tk := &Task{Title: "it-lock", AssigneeID: 3, Status: StatusDraft, Revision: 1}
	if err := repo.InsertTask(ctx, tk); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// revision 匹配 → 更新成功，RowsAffected=1。
	n, err := repo.UpdateStatus(ctx, tk.ID, 1, StatusAssigned)
	if err != nil || n != 1 {
		t.Fatalf("update matched: n=%d err=%v", n, err)
	}
	// 旧 revision 再更新 → 冲突（0 行受影响）。
	n2, err := repo.UpdateStatus(ctx, tk.ID, 1, StatusInProgress)
	if err != nil {
		t.Fatalf("update stale err: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("stale update should affect 0 rows, got %d", n2)
	}
	// 新 revision（2）更新成功。
	n3, err := repo.UpdateStatus(ctx, tk.ID, 2, StatusInProgress)
	if err != nil || n3 != 1 {
		t.Fatalf("update rev2: n=%d err=%v", n3, err)
	}
}

func TestGORMListByActorFilter(t *testing.T) {
	db := integrationDB(t)
	repo := newTxRepo(t, db)
	ctx := context.Background()

	_ = repo.InsertTask(ctx, &Task{Title: "t1", AssigneeID: 3, Status: StatusDraft, Revision: 1})
	_ = repo.InsertTask(ctx, &Task{Title: "t2", AssigneeID: 4, Status: StatusDraft, Revision: 1})
	_ = repo.InsertTask(ctx, &Task{Title: "t3", AssigneeID: 3, Status: StatusDraft, Revision: 1})

	all, err := repo.ListByActor(ctx, 3, true)
	if err != nil || len(all) != 3 {
		t.Fatalf("admin list: len=%d err=%v", len(all), err)
	}
	mine, err := repo.ListByActor(ctx, 3, false)
	if err != nil || len(mine) != 2 {
		t.Fatalf("inspector list: len=%d err=%v", len(mine), err)
	}
}

func TestGORMListPageAndCount(t *testing.T) {
	db := integrationDB(t)
	repo := newTxRepo(t, db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		st := StatusDraft
		if i%2 == 1 {
			st = StatusAssigned
		}
		_ = repo.InsertTask(ctx, &Task{Title: "pg", AssigneeID: 3, Status: st, Revision: 1})
	}
	// 分页第 2 页（page=2, size=2 → offset=2, limit=2）。
	page2, err := repo.ListPage(ctx, 3, true, ListFilter{}, 2, 2)
	if err != nil || len(page2) != 2 {
		t.Fatalf("page2: len=%d err=%v", len(page2), err)
	}
	// 状态过滤：ASSIGNED 共 2 条。
	assigned, err := repo.ListPage(ctx, 3, true, ListFilter{Status: StatusAssigned}, 0, 0)
	if err != nil || len(assigned) != 2 {
		t.Fatalf("assigned: len=%d err=%v", len(assigned), err)
	}
	total, err := repo.CountTasks(ctx, 3, true, ListFilter{Status: StatusAssigned})
	if err != nil || total != 2 {
		t.Fatalf("count assigned: %d err=%v", total, err)
	}
	totalAll, err := repo.CountTasks(ctx, 3, true, ListFilter{})
	if err != nil || totalAll != 5 {
		t.Fatalf("count all: %d err=%v", totalAll, err)
	}
}

func TestGORMCancelTask(t *testing.T) {
	db := integrationDB(t)
	repo := newTxRepo(t, db)
	ctx := context.Background()

	tk := &Task{Title: "it-cancel", AssigneeID: 3, Status: StatusDraft, Revision: 1}
	if err := repo.InsertTask(ctx, tk); err != nil {
		t.Fatalf("insert: %v", err)
	}
	n, err := repo.CancelTask(ctx, tk.ID, 1, "需求变更")
	if err != nil || n != 1 {
		t.Fatalf("cancel: n=%d err=%v", n, err)
	}
	got, err := repo.GetByID(ctx, tk.ID)
	if err != nil || got.Status != StatusCancelled || got.CancelReason != "需求变更" || got.CancelledAt == nil {
		t.Fatalf("after cancel: %+v err=%v", got, err)
	}
	// 已取消（终态）重复取消 → 0 行。
	n, err = repo.CancelTask(ctx, tk.ID, 1, "again")
	if err != nil || n != 0 {
		t.Fatalf("repeat cancel should hit 0: n=%d err=%v", n, err)
	}
}
