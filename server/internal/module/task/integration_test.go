//go:build integration

// GORM 仓储集成测试（真实 MySQL）。
//
// 前置：
//   1. 本机 MySQL 3306 运行；city_wall_test 库存在且 city_wall 用户可访问；
//   2. 迁移文件 001~003 已在测试库执行（可复用 cmd/migrate 或手动执行）。
// 运行：
//   cd server && go test -tags integration ./internal/module/task/ -v
// 隔离策略：每个用例在事务中执行，结束回滚，不污染库中数据。
package task

import (
	"context"
	"os"
	"path/filepath"
	"sort"
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

// runMigrations 在测试库执行 001~003 显式迁移（CREATE TABLE IF NOT EXISTS，幂等）。
func runMigrations(db *gorm.DB) error {
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "migrations", "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := sqlDB.Exec(string(raw)); err != nil {
			return err
		}
	}
	return nil
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

func TestGORMMarkInProgressOnlyWhenAssigned(t *testing.T) {
	db := integrationDB(t)
	repo := newTxRepo(t, db)
	ctx := context.Background()

	tk := &Task{Title: "it-mark", AssigneeID: 3, Status: StatusDraft, Revision: 1}
	if err := repo.InsertTask(ctx, tk); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// DRAFT 状态下标记不生效（保持 DRAFT）。
	if err := repo.MarkInProgressIfAssigned(ctx, tk.ID); err != nil {
		t.Fatalf("mark: %v", err)
	}
	got, _ := repo.GetByID(ctx, tk.ID)
	if got.Status != StatusDraft {
		t.Fatalf("draft should stay draft, got %s", got.Status)
	}
	// 派发后标记生效。
	if _, err := repo.UpdateStatus(ctx, tk.ID, 1, StatusAssigned); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := repo.MarkInProgressIfAssigned(ctx, tk.ID); err != nil {
		t.Fatalf("mark2: %v", err)
	}
	got, _ = repo.GetByID(ctx, tk.ID)
	if got.Status != StatusInProgress {
		t.Fatalf("assigned should become in_progress, got %s", got.Status)
	}
}
