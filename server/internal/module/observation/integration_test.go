//go:build integration

package observation

// GORM 集成测试：连接本机 MySQL（city_wall_test 库），验证真实 SQL 语义（乐观锁/快照/级联）。
// 运行：go test -tags integration ./internal/module/observation/
// DSN 可用环境变量 CITY_WALL_TEST_DSN 覆盖。

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

func newTxRepo(t *testing.T, db *gorm.DB) *GORMRepository {
	t.Helper()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin tx: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return NewGORMRepository(tx).(*GORMRepository)
}

func TestGORMInsertAndGetObservation(t *testing.T) {
	db := integrationDB(t)
	repo := newTxRepo(t, db)
	ctx := context.Background()

	o := &Observation{
		Source:            SourceVoluntary,
		ReporterID:        3,
		LocationText:      "it-loc",
		InspectionOutcome: OutcomeSuspected,
		Status:            ObsDraft,
		Revision:          1,
	}
	if err := repo.Insert(ctx, o); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if o.ID == 0 {
		t.Fatal("expected auto id")
	}
	got, err := repo.GetByID(ctx, o.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Source != SourceVoluntary || got.Status != ObsDraft || got.Revision != 1 {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestGORMUpdateStatusOptimisticLock(t *testing.T) {
	db := integrationDB(t)
	repo := newTxRepo(t, db)
	ctx := context.Background()

	o := &Observation{Source: SourceVoluntary, ReporterID: 3, LocationText: "lock", Status: ObsDraft, Revision: 1}
	if err := repo.Insert(ctx, o); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// 正确 revision 成功。
	n, err := repo.UpdateStatus(ctx, o.ID, 1, ObsSubmitted)
	if err != nil || n != 1 {
		t.Fatalf("update ok: n=%d err=%v", n, err)
	}
	// 过期 revision 冲突：0 行影响。
	n, err = repo.UpdateStatus(ctx, o.ID, 1, ObsAccepted)
	if err != nil || n != 0 {
		t.Fatalf("stale update should hit 0 rows: n=%d err=%v", n, err)
	}
}

func TestGORMImageLifecycle(t *testing.T) {
	db := integrationDB(t)
	repo := newTxRepo(t, db)
	ctx := context.Background()

	o := &Observation{Source: SourceVoluntary, ReporterID: 3, LocationText: "img", Status: ObsDraft, Revision: 1}
	if err := repo.Insert(ctx, o); err != nil {
		t.Fatalf("insert obs: %v", err)
	}
	img := &Image{StorageKey: "it/img_1.png", SHA256: "aa", MediaType: "image/png", ByteSize: 10, Width: 1, Height: 1, UploadedBy: 3}
	if err := repo.InsertImage(ctx, img); err != nil {
		t.Fatalf("insert img: %v", err)
	}
	if err := repo.LinkImage(ctx, o.ID, img.ID, ImageRoleOverview); err != nil {
		t.Fatalf("link: %v", err)
	}
	links, err := repo.ListImages(ctx, o.ID)
	if err != nil || len(links) != 1 || links[0].ImageID != img.ID || links[0].ImageRole != ImageRoleOverview {
		t.Fatalf("list links: %+v err=%v", links, err)
	}
	got, err := repo.GetImage(ctx, img.ID)
	if err != nil || got.StorageKey != "it/img_1.png" {
		t.Fatalf("get image: %+v err=%v", got, err)
	}
	// ObservationByImage 反查。
	own, err := repo.ObservationByImage(ctx, img.ID)
	if err != nil || own.ID != o.ID {
		t.Fatalf("obs by image: %+v err=%v", own, err)
	}
}

func TestGORMSubmissionSnapshot(t *testing.T) {
	db := integrationDB(t)
	repo := newTxRepo(t, db)
	ctx := context.Background()

	o := &Observation{Source: SourceVoluntary, ReporterID: 3, LocationText: "sub", Status: ObsDraft, Revision: 1}
	if err := repo.Insert(ctx, o); err != nil {
		t.Fatalf("insert: %v", err)
	}
	rev := &ConclusionRevision{ObservationID: o.ID, RevisionNo: 1, Outcome: OutcomeNormal, AuthorID: 3}
	item := ConclusionItem{Decision: "KEEP", FinalClassCode: "CRACK", Description: "it"}
	if err := repo.InsertConclusion(ctx, rev, []ConclusionItem{item}); err != nil {
		t.Fatalf("insert conclusion: %v", err)
	}
	latest, err := repo.LatestConclusion(ctx, o.ID)
	if err != nil || latest.RevisionNo != 1 {
		t.Fatalf("latest conclusion: %+v err=%v", latest, err)
	}

	no, err := repo.NextSubmissionNo(ctx, o.ID)
	if err != nil || no != 1 {
		t.Fatalf("next submission no: %d err=%v", no, err)
	}
	sub := &ObservationSubmission{ObservationID: o.ID, SubmissionNo: no, ConclusionRevisionID: &latest.ID, SubmittedAt: latest.CreatedAt}
	if err := repo.InsertSubmission(ctx, sub); err != nil {
		t.Fatalf("insert submission: %v", err)
	}
	got, err := repo.LatestSubmission(ctx, o.ID)
	if err != nil || got.SubmissionNo != 1 || got.ConclusionRevisionID == nil || *got.ConclusionRevisionID != latest.ID {
		t.Fatalf("latest submission: %+v err=%v", got, err)
	}
}
