// Command migrate 按版本顺序执行 migrations 目录下的显式 SQL 迁移。
//
// 用法：
//   go run ./cmd/migrate -config ../conf/server.json
//   go run ./cmd/migrate -config ../conf/server.json -dir ../server/migrations
//
// 幂等：schema_migrations 表记录已执行版本（文件名去扩展名），
// 已执行的文件自动跳过；执行失败的迁移会回滚当前文件并终止。
// 与 mysql 客户端执行方式等价，可在容器 entrypoint 中复用。
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql" // 注册 mysql 驱动

	"citywall/server/internal/config"
)

func main() {
	configPath := flag.String("config", "conf/server.json", "path to server config")
	dir := flag.String("dir", "migrations", "directory containing *.sql migration files")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	dsn := cfg.Database.DSN()
	if !strings.Contains(dsn, "multiStatements") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "multiStatements=true"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("open mysql: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping mysql: %v", err)
	}

	if err := ensureSchemaMigrations(db); err != nil {
		log.Fatalf("ensure schema_migrations: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(*dir, "*.sql"))
	if err != nil {
		log.Fatalf("glob migrations: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("no migration files found in %q", *dir)
	}
	sort.Strings(files)

	for _, f := range files {
		version := strings.TrimSuffix(filepath.Base(f), ".sql")
		done, err := isApplied(db, version)
		if err != nil {
			log.Fatalf("check %s: %v", version, err)
		}
		if done {
			log.Printf("migrate: %s already applied, skip", version)
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("read %s: %v", f, err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			log.Fatalf("apply %s: %v", version, err)
		}
		if _, err := db.Exec(
			"INSERT INTO schema_migrations (version, applied_at) VALUES (?, NOW(3))",
			version,
		); err != nil {
			log.Fatalf("record %s: %v", version, err)
		}
		log.Printf("migrate: %s applied", version)
	}
	log.Printf("migrate: all done (%d files)", len(files))
}

func ensureSchemaMigrations(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    VARCHAR(128) NOT NULL,
		applied_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		PRIMARY KEY (version)
	) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4`)
	return err
}

func isApplied(db *sql.DB, version string) (bool, error) {
	var n int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("query schema_migrations: %w", err)
	}
	return n > 0, nil
}
